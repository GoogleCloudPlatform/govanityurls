FROM scratch
copy $TRAVIS_BUILD_DIR/govanityurls /
ENTRYPOINT ["/govanityurls"]