---
title: The reverse proxy
description: Put Tuck behind nginx with TLS from certbot.
---

# The reverse proxy

Tuck only talks HTTPS, through a proxy you trust. Here's nginx with certbot.

1. Save this as `/etc/nginx/sites-available/tuck.conf`, with your domain:

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

2. Turn it on and add HTTPS:

   ```bash
   sudo ln -s /etc/nginx/sites-available/tuck.conf /etc/nginx/sites-enabled/
   sudo nginx -t && sudo systemctl reload nginx
   sudo certbot --nginx -d tuck.example.com
   ```

   Say yes when certbot offers to redirect to HTTPS.

3. Tell Tuck to trust the proxy, in `.env`:

   ```ini
   TUCK_TRUSTED_PROXIES=172.16.0.0/12
   ```

   Then `docker compose up -d` again.

:::tip still says "HTTPS only"?
Run `docker compose logs tuck | grep audit` to see the address requests come from, and put that in `TUCK_TRUSTED_PROXIES`.
:::
