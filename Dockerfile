FROM golang:1.20.6

# Set destination for COPY
WORKDIR /htmx-the-family-loop

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/engine/reference/builder/#copy
COPY *.go *.html *.js .env ./
RUN mkdir ./css ./js ./assets
COPY css/ ./css/
COPY js/ ./js/
COPY assets/ ./assets/
# Build

RUN CGO_ENABLED=0 GOOS=linux go build -o thefamilyloop .
#RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o thefamilyloop.exe .
RUN chmod 0755 thefamilyloop
# Run
EXPOSE 80
CMD ./thefamilyloop