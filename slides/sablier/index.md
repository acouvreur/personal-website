---
marp: true
theme: site
lang: en
paginate: true
title: Sablier, scale to zero
description: Start containers on demand and shut them down once nothing is using them, behind any reverse proxy.
author: Alexis Couvreur
header: Sablier, scale to zero
style: |
  /* Sablier's gold, for its name on the accent slides. */
  section.chapter .sablier { color: #ffd05b; }
  /* A whole sentence rather than a title. */
  section.statement h1 { font-size: 64px; line-height: 1.15; }
  section.statement blockquote p { font-size: 40px; line-height: 1.4; }
  /* Compose files don't fit at the theme's code size: tighter code, and more of
     the slide for it. */
  section.code { padding-top: 76px; padding-bottom: 40px; }
  section.code h2 { margin-bottom: .25em; }
  section.code h3 { font-size: 24px; margin: 0 0 .35em; }
  section.code .split pre { font-size: 16.5px; line-height: 1.32; margin-bottom: .9em; }
---

<!-- _class: lead -->
<!-- _paginate: false -->
<!-- _header: Montréal JUG · Lightning Présentations Éclairs, épisode 2 · 29 mars 2023 -->

![bg right:40%](img/cover.jpg)

# Sablier

## Scale to zero

[github.com/sablierapp/sablier](https://github.com/sablierapp/sablier)

---

<!-- _class: lead -->

# Alexis Couvreur

## Fullstack Software Engineer

Working at the National Bank of Canada for SII Canada

---

<!-- _class: lead statement -->

# Reducing infrastructure costs boils down to turning apps off when you don’t use them

---

<!-- _class: lead -->

# Scaling to zero

## Scaling to zero allows workloads to be run only when there is demand

---

## They scale to zero

<div class="split">

- Google Serverless (App Engine, Function)
- AWS Lambda
- Heroku
- Github Codespace
- And more!

![h:420](img/they-use-it.png)

</div>

---

<!-- _class: chapter -->

# <span class="sablier">Sablier</span>

A free and open source software scaling to zero solution

---

<!-- _class: chapter -->

# How does it work?

---

## Reverse proxy integration

![w:1088](img/reverse-proxy-integration.png)

---

<!-- _class: chapter -->

# How to use <span class="sablier">Sablier</span>?

Let’s see its integration with Caddy!

---

<!-- _class: code -->

## Without Sablier

<div class="split top">
<div>

### docker-compose.yaml

```yaml
services:
  proxy:
    image: caddy:2.6.4
    ports:
      - "8080:80"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro

  whoami:
    image: traefik/whoami:v1.12.0
```

</div>
<div>

### Caddyfile

```nginx
:80 {
  route /whoami {
    reverse_proxy whoami:80
  }
}
```

</div>
</div>

---

<!-- _class: code -->

## With Sablier

<div class="split top">
<div>

### docker-compose.yaml

```yaml
services:
  proxy:
    image: caddy:local
    ports:
      - "8080:80"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro

  whoami:
    image: traefik/whoami:v1.12.0
    labels:
      - sablier.enable=true
      - sablier.group=demo

  sablier:
    image: sablierapp/sablier:1.19.1
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
```

</div>
<div>

### Caddyfile

```nginx
:80 {
  route /whoami {
    sablier http://sablier:10000 {
      group demo
      session_duration 1m
      dynamic {
        display_name Dynamic-Whoami
      }
    }
    reverse_proxy whoami:80
  }
}
```

</div>
</div>

---

<!-- _class: chapter -->

# Demo

---

<!-- _class: chapter -->

# <span class="sablier">Sablier</span> features

---

## Providers

<div class="split">

- Docker
- Docker Swarm
- Kubernetes
- Podman
- AWS EC2

![h:420](img/providers.png)

</div>

---

<!-- _class: code -->

## Label filtering

<div class="split top">

- Docker labels
- Docker swarm service labels
- Kubernetes deployments labels

<div>

### Docker labels

```yaml
services:
  whoami:
    image: traefik/whoami:v1.12.0
    labels:
      - sablier.enable=true
      - sablier.group=mygroup
```

### Kubernetes deployments labels

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: whoami
  labels:
    app: whoami
    sablier.enable: "true"
    sablier.group: mygroup
```

</div>
</div>

---

## Reverse proxies

<div class="split">

- Traefik (Go)
- Caddy (Go)
- Nginx (Javascript/Lua/C)
- more to come...

![h:420](img/reverse-proxies-implem.png)

</div>

---

<!-- _class: lead -->

# Integration nightmare

## Develop and maintain an integration for each reverse proxy

---

![bg right:45% 85%](img/reverse-proxies.png)

- Different APIs
- Different languages (Go, C, Javascript, Lua)
  - Runtime plugin execution
  - Compilation plugin (recompile the proxy)
- Breaking changes
- Provider compatibility
- Cloud native

---

<!-- _class: chapter -->

# WebAssembly to the rescue

---

<!-- _class: lead statement -->

> WebAssembly (abbreviated Wasm) is a binary instruction format designed as a portable compilation target for programming languages, enabling deployment on the web for client and server applications.

---

## What is Web Assembly

- Bytecode (almost native speed)
- Compilation target for higher level languages (Rust, Go, C++...)
- Runs in a sandboxed environment

![h:290](img/webassembly.png)

---

<!-- _class: lead -->

# ProxyWasm

## WebAssembly modules for reverse proxies

---

![bg 70%](img/proxy-wasm.png)

---

<!-- _class: lead -->

# No more integration nightmare

## Create one WASM Module to be used by all reverse proxies

Reverse proxy maintainers will have to develop and maintain one ProxyWasm extension

---

<!-- _class: lead -->

# Thank you!

## Questions?
