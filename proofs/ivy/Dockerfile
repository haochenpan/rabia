FROM debian:buster

RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections
RUN apt-get update && apt-get install -y apt-utils

# Install and configure locale `en_US.UTF-8`
RUN apt-get update && apt-get install -y locales && \
    sed -i -e "s/# $en_US.*/en_US.UTF-8 UTF-8/" /etc/locale.gen && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8
ENV LANG=en_US.UTF-8

RUN apt-get update && apt-get install -y python2 python-pip python-ply python-tk tix pkg-config sudo python-setuptools
RUN apt-get update && apt-get install -y libreadline-dev 

# create a user:
RUN useradd -ms /bin/bash user && echo 'user:user' | chpasswd && adduser user sudo
RUN echo "ALL ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers
USER user
WORKDIR /home/user

# install Ivy:
RUN (sudo apt-get update; \
     sudo apt-get install -y g++ cmake git; \
     git clone --recurse-submodules https://github.com/kenmcmil/ivy.git; \
     cd ivy/ && python build_submodules.py; \
     sudo apt-get remove -y g++ cmake git; \
     sudo apt-get clean; \
     rm -rf ivy/.git; \
     rm -rf ivy/submodules)
WORKDIR /home/user/ivy/
RUN mkdir -p "/home/user/python/lib/python2.7/site-packages"
ENV PYTHONPATH="/home/user/python/lib/python2.7/site-packages"
RUN pip install 'pyparsing==2.1.4'
RUN python2.7 setup.py develop --prefix="/home/user/python/"
ENV PATH=$PATH:"/home/user/python/bin/"

RUN pip install pexpect chardet

COPY weak_mvc.ivy /home/user/
WORKDIR /home/user/
