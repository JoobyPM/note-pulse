/**
 * reactive-store.js
 * A tiny reactive signals library (ESM, no classes).
 *
 * @module reactive-store
 */

/**
 * @template T
 * @typedef {Object} Signal
 * @property {() => T} get Read the current value.
 * @property {(value: T | ((prev: T) => T)) => void} set Update the value (function updater allowed).
 * @property {(listener: (value: T) => void) => () => void} subscribe Subscribe to value changes.
 */

/**
 * Create a reactive signal.
 *
 * @template T
 * @param {T} initial Initial value.
 * @returns {Signal<T>}
 */
function createSignal(initial) {
	let value = initial;
	/** @type {Set<(v: T) => void>} */
	const listeners = new Set();

	/** @returns {T} */
	function get() {
		return value;
	}

	/**
	 * @param {T | ((prev: T) => T)} next
	 */
	function set(next) {
		/** @type {T} */
		const nextVal = typeof next === 'function' ? /** @type {(prev:T)=>T} */ (next)(value) : next;
		if (Object.is(nextVal, value)) return;
		value = nextVal;
		listeners.forEach((l) => l(value));
	}

	/**
	 * @param {(v: T) => void} listener
	 * @returns {() => void} unsubscribe function
	 */
	function subscribe(listener) {
		listeners.add(listener);
		listener(value); // emit current value immediately
		return () => listeners.delete(listener);
	}

	return { get, set, subscribe };
}

/**
 * Register a side effect that re‑runs when dependencies emit.
 *
 * @param {() => void} callback Effect function.
 * @param {Array<Signal<unknown>>} [dependencies=[]] Signals to track.
 * @returns {() => void} dispose function.
 */
function createEffect(callback, dependencies = []) {
	const unsubs = dependencies.map((dep) => dep.subscribe(callback));
	callback(); // run once at setup
	return () => unsubs.forEach((u) => u());
}

/**
 * @template T
 * @typedef {Object} Computed
 * @property {() => T} get Read the cached value.
 * @property {(listener: (v:T)=>void) => () => void} subscribe Subscribe to updates.
 * @property {() => void} dispose Stop tracking dependencies and listeners.
 */

/**
 * Make a cached, automatically updated computation.
 *
 * @template T
 * @param {() => T} compute Function producing the value.
 * @param {Array<Signal<unknown>>} [dependencies=[]] Signals to track.
 * @returns {Computed<T>}
 */
function createComputed(compute, dependencies = []) {
	let cached = compute();
	/** @type {Set<(v:T)=>void>} */
	const listeners = new Set();

	function evaluate() {
		const next = compute();
		if (Object.is(next, cached)) return;
		cached = next;
		listeners.forEach((l) => l(cached));
	}

	const unsubs = dependencies.map((dep) => dep.subscribe(evaluate));

	/** @returns {T} */
	function get() {
		return cached;
	}

	/**
	 * @param {(v:T)=>void} listener
	 * @returns {() => void}
	 */
	function subscribe(listener) {
		listeners.add(listener);
		listener(cached);
		return () => listeners.delete(listener);
	}

	function dispose() {
		unsubs.forEach((u) => u());
		listeners.clear();
	}

	return { get, subscribe, dispose };
}

/**
 * @template S
 * @typedef {Object} Store
 * @property {() => S} get Read the current state snapshot.
 * @property {(patch: Partial<S> | ((prev:S)=>S)) => void} set Merge a patch or apply a functional update.
 * @property {(listener: (state:S)=>void) => () => void} subscribe Subscribe to state changes.
 */

/**
 * Shallow object store built on createSignal.
 *
 * @template S extends object
 * @param {S} [initialObject={}] Optional initial state.
 * @returns {Store<S>}
 */
function createStore(initialObject = /** @type {S} */ ({})) {
	const base = createSignal({ ...initialObject });

	/**
	 * Update state.
	 * @param {Partial<S> | ((prev:S)=>S)} patch
	 */
	function set(patch) {
		base.set((prev) =>
			typeof patch === 'function' ? /** @type {(prev:S)=>S} */ (patch)(prev) : { ...prev, ...patch }
		);
	}

	return { get: base.get, set, subscribe: base.subscribe };
}

export { createComputed, createEffect, createSignal, createStore };

/* -------------------------------------------------------------------------- */
/* Example usage (comment out or remove in production)                         */
/* -------------------------------------------------------------------------- */

/*
  import {
    createSignal,
    createComputed,
    createEffect,
    createStore
  } from "./reactive-store.js";

  const count = createSignal(0);
  const doubled = createComputed(() => count.get() * 2, [count]);

  createEffect(
    () => console.log(`count: ${count.get()}, doubled: ${doubled.get()}`),
    [count, doubled]
  );

  count.set(1);          // logs: count: 1, doubled: 2
  count.set(v => v + 1); // logs: count: 2, doubled: 4

  const store = createStore({ a: 1, b: 2 });
  store.subscribe(state => console.log("store", state));
  store.set({ b: 3 });                          // logs: { a:1, b:3 }
  store.set(prev => ({ ...prev, c: 4 }));       // logs: { a:1, b:3, c:4 }
  */
