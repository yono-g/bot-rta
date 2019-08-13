## How to develop

1. `$ docker-compose build dev`
1. `$ docker-compose run --rm dev dep ensure`
1. `$ docker-compose up`

## How to deploy

1. `$ docker-compose exec dev sh`
1. `# gcloud auth login`
1. `# gcloud config set project PROJECT_ID`
1. `# gcloud app deploy gae/app.yaml`
