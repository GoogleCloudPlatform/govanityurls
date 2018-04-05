FROM scratch
copy $TRAVIS_BUILD_DIR/govanityurls /
RUN chmod +x /govanityurls
ENTRYPOINT ["/govanityurls"]