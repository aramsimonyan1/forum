# Use the official Golang image:    or?: FROM golang:1.22.5
FROM golang:latest

# Set the Current Working Directory inside the container:
WORKDIR /app

# Copy everything from the current directory to the PWD (Present Working Directory) inside the container:    or?: COPY . /app
COPY . .

# Install SQLite and related dependencies:
#RUN apt-get update && apt-get install -y sqlite3 libsqlite3-dev

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed:
#RUN go mod download

# Build the Go app:                 or?: RUN go build -o main
RUN go build -o forum main.go

# Ensure the executable has the correct permissions:
#RUN chmod +x forum

# Expose the application port:
EXPOSE 8080

# Command to run the executable:    or?: CMD ["/app/main"]
CMD ["/app/forum"]
