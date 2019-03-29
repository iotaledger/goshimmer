package database

import (
    "github.com/iotaledger/goshimmer/packages/parameter"
)

var DIRECTORY = parameter.AddString("DATABASE/DIRECTORY", "mainnetdb", "path to the database folder")
