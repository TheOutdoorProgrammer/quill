# Fixture for the CI smoke test. Quill ships no image of its own; this exists
# to give the Docker publisher something real to build, and to fail the build
# if the version and commit build arguments never arrive.
FROM alpine:3.22

ARG VERSION
ARG COMMIT

RUN test -n "$VERSION" || (echo "VERSION build-arg was empty" && exit 1)
RUN test -n "$COMMIT" || (echo "COMMIT build-arg was empty" && exit 1)
RUN echo "built $VERSION at $COMMIT"
