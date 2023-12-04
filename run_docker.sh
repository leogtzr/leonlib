#!/bin/bash

export LEONLIB_DB_PASSWORD=${LEONLIB_DB_PASSWORD}
export LEONLIB_DB_USER="leo"
export LEONLIB_DB="leonlib"
export LEONLIB_DB_HOST=leonlib
export DB_MODE="inmemory"
export PORT=8180
export PGPORT=5432

docker-compose up --build

exit
