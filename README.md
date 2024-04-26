# What is this?

**[Gitea](https://about.gitea.com/products/gitea/) is a self-hostable Git hosting platform**. Like GitHub or GitLab, you can host projects that use the Git version control system there, and it also offers some related functionality like issue tracker, package repository, wikis, etc.

This repository offers a solution to **deploy static websites from a hosted repository**. It automatically creates and updates folders from the `gitea-pages` branches of every repository that has one.

Contrary to other solutions for static pages, this one doesn't try to actually serve the pages, but only copies the content. You can **use your existing webserver to serve the files** any way you see fit.

# Installation

## Configure Gitea

First, generate a secret token, for example `secret-token-please-replace`.

On your Gitea instance, go to the admin settings > integrations > webhooks. Click on the button "Add Webhook" next to "System Webhooks" then select "Gitea".

In the "target URL", put the location and port where you will be running gitea-pages-static, for example `http://localhost:3000/webhook`. Make sure you pick a port that is not used by another service.

Select "custom events" and check "create", "delete", "push", and "repository".

In "branch filter", enter `Bearer secret-token-please-replace`.

Then click "Add Webhook".

## Run gitea-pages-static

Download the binary from this project, then run it with environment variables set to match your situation:

```console
$ GITEA_PAGES_REPOSITORIES=/var/lib/gitea/data/repository \
  GITEA_PAGES_TARGET=/var/www/git-pages \
  GITEA_PAGES_LISTEN_ADDR=127.0.0.1:3000 \
  GITEA_PAGES_TOKEN=secret-token-please-replace \
  ./gitea-pages-static
```

## Configure your webserver

All you have to do now is configure your webserver to serve `/var/www/git-pages` (or whatever target you set).

# Alternatives

[The awesome-gitea project lists alternatives](https://gitea.com/gitea/awesome-gitea#web-hosting). Feel free to check whether some of them might suit your needs.
