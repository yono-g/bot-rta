FROM gcr.io/gcpug-container/appengine-go:alpine

RUN mkdir -p /go/bin && \
    curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
