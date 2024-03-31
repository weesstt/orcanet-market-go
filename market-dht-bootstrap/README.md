# Starfish Market DHT Bootstrap 
This is a minimal DHT node for the orcanet market. These nodes are only capable of starting/joining the network and discovering/connecting to other peers on the market. These nodes will run in server mode and must have a public IP address to allow connections.

## Options
```
-bootstrap: Multiaddr of other bootstrap peer to connect to. 
```

## Example Network Setup

1) Start a bootstrap node (must have public ip) to start network.

    ```go run .```

2) Connect a second bootstrap node (must have public ip) to the first one.

    ```go run . -bootstrap [multiAddr]```

3) Connect a thrid bootstrap node (must have public ip) to the second one.

    ```go run . -bootstrap [multiAddr]```