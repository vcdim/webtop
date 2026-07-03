# webtop

Lightweight, single-binary web-based system dashboard. Real-time CPU, memory, port, and GPU monitoring in your browser.

![system](assets/system.png)
![ports](assets/ports.png)
![gpu](assets/gpu.png)

## Install

```
curl -fsSL https://vcdim.github.io/webtop/install.sh | sudo bash
```

The service starts automatically after install. Open http://localhost:9999.

## Usage

Use `-p` to change port, `-i` to change refresh interval:

```
sudo webtop -p 8080 -i 5s
```

Manage the service:

```
sudo systemctl restart webtop
```

```
sudo systemctl stop webtop
```

## Uninstall

```
curl -fsSL https://vcdim.github.io/webtop/uninstall.sh | sudo bash
```

## Build from Source

```
go build -o webtop .
```

## License

MIT
