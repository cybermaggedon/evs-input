
FROM fedora:32

ARG EVS_API=master

COPY evs-input /usr/local/bin/evs-input

ENV PULSAR_BROKER=pulsar://exchange:6650
ENV METRICS_PORT=8088
EXPOSE 8088

CMD /usr/local/bin/evs-input

