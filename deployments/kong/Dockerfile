FROM kong:3.8-ubuntu
COPY plugins/caller-context /usr/local/share/lua/5.1/kong/plugins/caller-context
ENV KONG_PLUGINS=bundled,caller-context
