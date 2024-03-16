# Starfish Market DHT

## Usage
```go run .```
- Options:
    - bootstrap: Provide a bootstrap address to connect to.
    - clientMode: Run node in client mode, can only query DHT. Default to server mode.
    - searchKey: Provide a key to repeated search the DHT for, will print out when value found.
    - putKey: Provide a key to put a value into the DHT, a putValue must be specified as well.
    - putValue: Provide a value to put into for the specified key into the DHT, must specify putKey as well.

## Example Network Setup

1) Start a bootstrap node (must have public ip) to start network.

    ```go run .```

2) Connect a second bootstrap node (must have public ip) to the first one.

    ```go run . -bootstrap [multiAddr]```

3) Connect a thrid bootstrap node (must have public ip) to the second one.

    ```go run . -bootstrap [multiAddr]```

4) Now a client node can be started. This node can be provided with either three of the above
    bootstrap node multiAddr to connect to network.
    
    - Dummy Client

        ```go run . -clientMode -bootstrap [multiaddr]``` 
    - Client node to put value into DHT

        ```go run . -clientMode -bootstrap [multiaddr] -putKey myKey -putValue myValue``` 
    - Client node to search value in DHT

        ```go run . -clientMode -bootstrap [mutliaddr] -searchKey myKey```
