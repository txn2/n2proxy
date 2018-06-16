![n2proxy](mast.jpg)

# n2proxy

Contraband filtering reverse proxy.

**Install**
```bash
# download sample configuration file
wget https://raw.githubusercontent.com/txn2/n2proxy/master/cfg.yml

# install on mac
brew install txn2/tap/n2proxy

# upgrade
brew upgrade n2proxy
```

**Use**
```bash
# get the version
n2proxy --version

# get help
n2proxy --help

# environment variable override defaults
CFG=./cfg.yml PORT=9090 BACKEND=http://example.com:80 n2proxy

# command line options override environment variables
n2proxy --port=9091 --backend=http://example.com:80


# docker
docker run --rm -t -v "$(pwd)":/cfg/ -p 9092:9092 \
    txn2/n2proxy --port=9092 --cfg=/cfg/cfg.yml \
    --backend=http://example.com

```

Browse to http://localhost:9090
