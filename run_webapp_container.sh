#!/bin/bash

export LEONLIB_DB_PASSWORD=${LEONLIB_DB_PASSWORD}
export LEONLIB_DB_USER="leo"
export LEONLIB_DB="leonlib"
export LEONLIB_DB_HOST=leonlib
export PORT=8180
export PGPORT=5432
export LEONLIB_CAPTCHA_SITE_KEY=${LEONLIB_CAPTCHA_SITE_KEY}
export LEONLIB_MAINAPP_USER=${LEONLIB_MAINAPP_USER}

# docker-compose -f docker-compose.yml up --build
docker-compose up --no-deps --build app

exit
