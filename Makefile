all: 
	go build -o snowcast_control ./cmd/snowcast_control/main.go
	go build -o snowcast_listener ./cmd/snowcast_listener/main.go
	go build -o snowcast_server ./cmd/snowcast_server/main.go
clean:
	rm ./snowcast_control
	rm ./snowcast_listener
	rm ./snowcast_server