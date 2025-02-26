# cesium-go

## Description

`cesium-go` is a reverse proxy server with caching capabilities, designed to serve tiles from Google Maps API. It uses BadgerDB for caching responses to improve performance and reduce redundant API calls.

## Installation

1. Clone the repository:
    ```sh
    git clone https://github.com/tbxark/cesium-go.git
    cd cesium-go
    ```

2. Install dependencies:
    ```sh
    go mod download
    ```

3. Build the project:
    ```sh
    go build -o cesium-go
    ```

## Configuration

Create a `config.json` file with the following structure:

```json
{
  "address": ":8080",
  "cache_dir": "./cache",
  "api_keys": ["YOUR_GOOGLE_MAPS_API_KEY"]
}
```

- `address`: The address on which the server will listen.
- `cache_dir`: The directory where the cache will be stored.
- `api_keys`: A list of Google Maps API keys to be used by the proxy.

## Usage

Start the server with the configuration file:

```sh
./cesium-go -config=config.json
```

## Endpoints

- `GET /`: Serves the `index.html` file.
- `GET /{path}`: Proxies the request to Google Maps API and caches the response.

## License

This project is licensed under the MIT License.

