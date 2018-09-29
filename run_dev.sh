#!/usr/bin/env bash

dev_appserver.py \
  --datastore_path=.datastore \
  --clear_datastore=yes \
  --clear_search_indexes=yes \
  app.yaml
