#! /bin/bash

set -e

if [ -d "api-generator" ]; then
  cd api-generator
fi

GENERATOR=openapi-generator-cli.jar

API_MODEL_PACKAGE_NAME=apimodel

java -jar $GENERATOR generate \
  -i ../api/openapi-v3-spec.json \
  -o tmp/$API_MODEL_PACKAGE_NAME \
  --package-name $API_MODEL_PACKAGE_NAME \
  --global-property modelTests=false,modelDocs=false,apiTests=false,apiDocs=false,generateClient=false \
  -g go

mkdir -p ../api
mv tmp/$API_MODEL_PACKAGE_NAME/generated_models.go ../api/generated_apimodel.go || (rm -rf tmp && exit 1)
rm -rf tmp
