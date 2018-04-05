FROM scratch
copy $TRAVIS_BUILD_DIR/govanityurls $TRAVIS_BUILD_DIR/app.yaml $TRAVIS_BUILD_DIR/vanity.yaml /
ENTRYPOINT ["/govanityurls"]
CMD [ "vanity.yaml" ]