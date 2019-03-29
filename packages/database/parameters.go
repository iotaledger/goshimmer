package database

import (
    "github.com/iotadevelopment/go/modules/parameter"
)

var DIRECTORY = parameter.AddString("DATABASE/DIRECTORY", "mainnetdb", "path to the database folder")
