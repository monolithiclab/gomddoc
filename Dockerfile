# Image is built by GoReleaser. The binary is cross-compiled outside this
# Dockerfile (per target platform) and made available in the build context
# as `gomddoc`. Theme assets are embedded in the binary via //go:embed, so
# no extra files are needed at runtime.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99

COPY gomddoc /gomddoc

EXPOSE 8080

ENTRYPOINT ["/gomddoc"]
