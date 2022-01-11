FROM centos

RUN yum -y install net-tools
COPY ./webhook-server /
ENTRYPOINT [ "/webhook-server" ]