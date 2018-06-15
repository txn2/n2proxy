# n2proxy

Contraband filtering reverse proxy.

**Install**
```bash
# download sample configuration file
wget https://raw.githubusercontent.com/txn2/n2proxy/master/cfg.yml

# install on mac
brew install txn2/tap/n2proxy
```

**Use**
```bash
CFG=./cfg.yml PORT=9090 BACKEND=http://example.com:80 n2proxy
```

Browse to http://localhost:9090
