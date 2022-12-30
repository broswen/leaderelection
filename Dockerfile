FROM golang:1.19.4 AS build
# run build process in /app directory
WORKDIR /app
# copy dependencies and get them
COPY ./go.mod ./go.sum ./
#RUN go get -d -v ./...
RUN go mod download
# copy go src file(s)
COPY ./main.go ./main.go
# build the binary
RUN GOOS=linux CGO_ENABLED=1 GOARCH=amd64 go build -o leader ./main.go

FROM golang:1.19.4
# copy the binary from build stage
COPY --from=build /app/leader /bin/leader
# use non root
USER 1000:1000
# start server
CMD ["/bin/leader"]