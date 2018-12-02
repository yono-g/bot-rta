#!/usr/bin/env bash

dev_appserver.py \
  --datastore_path=.local-datastore.bin \
  --clear_datastore=yes \
  --clear_search_indexes=yes \
  ./gae
