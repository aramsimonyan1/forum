# docker build -t my-forum-app
# docker image ls

# Run the Docker container:
 #docker run -p 8080:8080 my-forum-app

# Forcefully remove all containers (stopping them if necessary)
  #docker rm -f $(docker ps -a -q)

# Verify Containers Are Removed
  #docker ps -a

# or From golang:1.22.5
FROM golang:latest

WORKDIR /app

# or COPY . /app
COPY . .

# Install SQLite and related dependencies
# RUN apt-get update && apt-get install -y sqlite3 libsqlite3-dev

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# or RUN go build -o main
RUN go build -o forum main.go

# Ensure the executable has the correct permissions
RUN chmod +x forum

# Expose the application port
EXPOSE 8080

#or CMD ["/app/main"]
CMD ["/app/forum"]

