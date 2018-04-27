# build stage
FROM golang:alpine AS build-env
ADD . /src
RUN cd /src && go build -o achsvc

# final stage
FROM alpine
WORKDIR /moov
COPY --from=build-env /src/achsvc /moov/
ENTRYPOINT ./achsvc
EXPOSE 80