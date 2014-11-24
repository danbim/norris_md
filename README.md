# NorrisMd

NorrisMd is a simple, yet extremely powerful WYSIWYGWS (what you see is what you get when saving)
Web site editor and generator. While creating and editing your site using Markdown syntax NorrisMd
runs an HTTP server pushing your edits to the browser whenever you press 'save', giving you an
almost live view of the website while creating it.

In addition NorrisMd let's you generate static Web sites from your markdown content tree ready to
be served as static content by any Web server of your choice!

## Usage

NorrisMd is currently in its early stages. It is built using the Go programming language on server
side and a bit of JavaScript on client side to provide live updates while editing your site. To run
NorrisMd, check out the project and run

```
go build
./norris_md
```

## Why the name?

Go ask Chuck if you dare to!

## Roadmap

### Live editing features

 * [X] Allow configuration of port and hostname
 * [X] Allow configuration of document root directory
 * [ ] Allow attaching meta data to documents
       * [X] title attribute to control the displayed title in menu
       * [ ] site welcome page (Home.md shouldn't be visible but displayed when clicking on home)
       * [ ] keywords for search engines, author data, ...
 * [X] Only render .md files, serve other static contents as well
 * [X] Ignore hidden files (e.g., vim swap files)
 * [ ] Allow sorting (sub)pages in the menu bar
 * [ ] Document how to configure NorrisMd and the site (using meta data files)
 * [ ] Include static content into the binary built by the Go compiler (using e.g. go-bindata)

### Site generator

 * [ ] Build basic site generation feature
 * [ ] Sanity checker
       * [ ] check links between .md sites
       * [ ] check links and embeds to/of static assets (e.g. pictures)
