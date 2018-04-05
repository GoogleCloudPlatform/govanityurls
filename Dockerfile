FROM scratch
copy govanityurls /
RUN chmod +x /govanityurls
ENTRYPOINT ["/govanityurls"]