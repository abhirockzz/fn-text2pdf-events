FROM fnproject/go:dev as build-stage
WORKDIR /function
RUN go get -u github.com/golang/dep/cmd/dep
ADD . /go/src/func/
RUN cd /go/src/func/ && dep ensure
RUN cd /go/src/func/ && go build -o func

FROM fnproject/go
WORKDIR /function
COPY --from=build-stage /go/src/func/func /function/
# Add OCI private key for OCI Go SDK authentication (OCI object storage API)
ARG PRIVATE_KEY_NAME
COPY $PRIVATE_KEY_NAME /function/$PRIVATE_KEY_NAME
ENTRYPOINT ["./func"]
