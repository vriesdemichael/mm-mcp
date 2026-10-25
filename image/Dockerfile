# The container image (ADR-023): the Linux binary GoReleaser built, on
# distroless's static base, which holds the certificate authorities, the time
# zones and a user that is not root, and no shell or package manager.
#
# Its context is dist/ after a GoReleaser run, for linux/amd64 and linux/arm64;
# Dockerfile.dockerignore lets in the Linux binaries alone. The release
# workflow builds it for both and pushes them under one tag, and
# `task release:image` builds it from a snapshot. Nothing in it runs a command
# at build time, so building for the other architecture needs no emulator.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99

ARG TARGETARCH
# The release, without its v, and the commit it was built from.
ARG VERSION=0.0.0
ARG REVISION=unknown

# io.modelcontextprotocol.server.name is how the MCP Registry knows the image
# is this server's: it must be server.json's name.
LABEL org.opencontainers.image.title="mm-mcp" \
      org.opencontainers.image.description="An MCP server for Mattermost." \
      org.opencontainers.image.url="https://vriesdemichael.github.io/mm-mcp/" \
      org.opencontainers.image.source="https://github.com/vriesdemichael/mm-mcp" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      io.modelcontextprotocol.server.name="io.github.vriesdemichael/mm-mcp"

# A container's disk is not the person's: a file saved there is lost with the
# container, and a path names nothing they have. MM_MCP_LOCAL_FILES=true, with
# a directory mounted and MM_MCP_DOWNLOAD_DIR naming it, brings save_file and
# attaching from a path back (ADR-029).
ENV MM_MCP_LOCAL_FILES=false

COPY --chmod=0555 mm-mcp_linux_${TARGETARCH}_*/mm-mcp /usr/bin/mm-mcp

# distroless's nonroot user, by number, so a runtime that refuses root can tell.
USER 65532:65532
ENTRYPOINT ["/usr/bin/mm-mcp"]
CMD ["serve"]
