################################################################################################
#####
##### DEV
#####
################################################################################################
FROM golang:1.10-alpine AS dev

#########################################################
## Install build dependencies
#########################################################

# program variables 
ARG PROG_REPO_VCS=${PROG_REPO_VCS:-"github.com"}
ARG PROG_REPO_OWNER=${PROG_REPO_OWNER:-"sniperkit"}
ARG PROG_REPO_NAME=${PROG_REPO_NAME:-"mysqldump-loader"}
ARG PROG_REPO_URI=${PROG_REPO_URI:-"${PROG_REPO_VCS}/${PROG_REPO_OWNER}/${PROG_REPO_NAME}"}
ARG PROG_REPO_URI_ABS=${PROG_REPO_URI_ABS:-"${GOPATH}/${PROG_REPO_VCS}/${PROG_REPO_OWNER}/${PROG_REPO_NAME}"}

# binaries build variables
ARG PROG_DEPS_MGR=${PROG_DEPS_MGR:-"glide"}

# apk
ARG APK_DEV=${APK_DEV:-"make cmake mercurial jq nano bash musl-dev openssl ca-certificates"}
RUN apk add --no-cache --no-progress ${APK_DEV}

#########################################################
## Copy files in the container & build targets
#########################################################

COPY .  ${PROG_REPO_URI_ABS}
WORKDIR ${PROG_REPO_URI_ABS}

RUN cd ${PROG_REPO_URI_ABS} \
 	&& make deps-all-glide

################################################################################################
#####
##### BUILDER
#####
################################################################################################
FROM golang:1.10-alpine AS builder

#########################################################
## Install build dependencies
#########################################################

ARG APK_BUILDER=${APK_BUILDER:-"make cmake mercurial openssl ca-certificates"}
RUN apk add --no-cache --no-progress ${APK_BUILDER}

COPY .  ${PROG_REPO_URI_ABS}
WORKDIR ${PROG_REPO_URI_ABS}

#########################################################
## Copy files in the container & build targets
#########################################################

RUN cd ${PROG_REPO_URI_ABS} \
 	&& make build

################################################################################################
#####
##### RUNTIME
#####
################################################################################################
FROM alpine:3.7 AS runtime

#########################################################
## Install build dependencies
#########################################################

# tini
ARG TINI_VCS_URI=${TINI_VCS_URI:-"github.com/krallin/tini"}
ARG TINI_VERSION=${TINI_VERSION:-"0.17.0"}
ARG TINI_COMPILER=${TINI_COMPILER:-"muslc"}
ARG TINI_ARCH=${TINI_ARCH:-"amd64"}
ARG TINI_PATH=${TINI_PATH:-"/usr/local/sbin/tini"}

# Install tini to /usr/local/sbin
ADD https://${TINI_VCS_URI}/releases/download/v${TINI_VERSION}/tini-${TINI_COMPILER}-${TINI_ARCH} ${TINI_PATH}

# apk 
ARG APK_RUNTIME=${APK_RUNTIME:-"ca-certificates git libssh2 openssl"}

# global
ARG OPT_DIR=${OPT_DIR:-"/opt"}

# program(s)
ARG PROG_NAME=${PROG_NAME:-"mysqldump-loader"}
ARG PROG_HOME=${PROG_HOME:-"${OPT_DIR}/${PROG_NAME}"}
ARG PROG_USER=${PROG_USER:-"mydl"}
ARG PROG_DATA=${PROG_DATA:-"${PROG_HOME}/shared/data/docker"}

# Install runtime dependencies & create runtime user
RUN apk --no-cache --no-progress add ${APK_RUNTIME} \
	&& chmod +x ${TINI_PATH} \
	&& mkdir -p ${OPT_DIR} \
	&& adduser -D ${PROG_USER} -h ${PROG_HOME} -s /bin/sh \
	&& su ${PROG_USER} -c 'cd ${PROG_HOME}; mkdir -p bin config data'

# Switch to user context
USER ${PROG_USER}
WORKDIR ${PROG_HOME}

#########################################################
## Copy files in the container & build targets
#########################################################

# Copy git2etcd binary to /opt/${PROG_HOME}/bin
COPY --from=builder ${PROG_REPO_URI_ABS}/bin ${PROG_HOME}/bin
COPY ./shared/config ${PROG_HOME}/config

ENV PATH $PATH:${PROG_HOME}/bin

# Container configuration
EXPOSE 3306
VOLUME ["${PROG_DATA}"]
ENTRYPOINT ["tini", "-g", "--"]
CMD ["/opt/mydl/bin/mysqldump-loader"]

