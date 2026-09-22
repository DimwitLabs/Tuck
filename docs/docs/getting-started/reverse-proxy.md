---
title: the reverse proxy
description: Put Tuck behind nginx with TLS from certbot.
---

# the reverse proxy

tuck only talks https, through a proxy you trust. here's nginx with certbot.

1. save this as `/etc/nginx/sites-available/tuck.conf`, with your domain:

   ```nginx
   server {
       listen 80;
       listen [::]:80;
       server_name tuck.example.com;

       client_max_body_size 32m;

       location / {
           proxy_pass http://127.0.0.1:8080;
           proxy_http_version 1.1;
           proxy_set_header Host              $host;
           proxy_set_header X-Forwarded-For   $remote_addr;
           proxy_set_header X-Forwarded-Proto $scheme;
           proxy_set_header Connection        "";
       }
   }
   ```

2. turn it on and add https:

   ```bash
   sudo ln -s /etc/nginx/sites-available/tuck.conf /etc/nginx/sites-enabled/
   sudo nginx -t && sudo systemctl reload nginx
   sudo certbot --nginx -d tuck.example.com
   ```

   say yes when certbot offers to redirect to https.

3. tell tuck to trust the proxy, in `.env`:

   ```ini
   TUCK_TRUSTED_PROXIES=172.16.0.0/12
   ```

   then `docker compose up -d` again.

:::tip still says "https only"?
run `docker compose logs tuck | grep audit` to see the address requests come from, and put that in `TUCK_TRUSTED_PROXIES`.
:::
