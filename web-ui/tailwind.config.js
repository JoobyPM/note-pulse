/** @type {import('tailwindcss').Config} */
export default {
  content: ["./src/**/*.{html,js,ts,jsx,tsx,md,mdx}"],
  darkMode: "class",
  theme: {
    extend: {
      // Custom border-3 utility for thicker focus indicators (accessibility)
      borderWidth: {
        "3": "3px",
      },
    },
  },
  plugins: [],
};
