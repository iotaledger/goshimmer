#!/bin/sh

# If MongoDB is enabled, wait for it to start
if [ "$MONGO_DB_ENABLED" = "true" ]
then
	/run/wait-for.sh mongodb:27017 -- $*
else 
	$*
fi
