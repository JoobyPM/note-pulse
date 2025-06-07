package mongo

import "go.mongodb.org/mongo-driver/v2/bson"

// ExistsFalse is a reusable shortcut for {$exists:false}.
var ExistsFalse = bson.M{"$exists": false}
