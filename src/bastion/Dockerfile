FROM ubuntu:22.04 AS dist
ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /root

# General tooling.
RUN apt update && apt install -y \
    apt-transport-https \
    bison \
    build-essential \
    ca-certificates \
    curl \
    git \
    gnupg \
    jq \
    mercurial \
    mitmproxy \
    postgresql-client \
    software-properties-common \
    tmux \
    vim \
    wget

# atlas (outil de migration de la base unifiée — voir .ia/ARCHITECTURE.md §6.4)
RUN curl -sSf https://atlasgo.sh | sh -s -- --yes

# nodejs
RUN curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
RUN echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_20.x nodistro main" | tee /etc/apt/sources.list.d/nodesource.list
RUN apt update && apt install -y nodejs

# golang
RUN curl -sLO https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
RUN tar -x -z -f go1.21.0.linux-amd64.tar.gz -C /usr/local/ --one-top-level
ENV PATH="${PATH}:/usr/local/go1.21.0.linux-amd64/go/bin"
RUN rm go1.21.0.linux-amd64.tar.gz

CMD tail -f /dev/null