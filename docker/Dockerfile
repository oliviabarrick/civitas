FROM quay.io/footloose/debian10

RUN apt-get update && apt-get install -y apt-transport-https curl gnupg2 software-properties-common && \
    wget -O - https://packages.cloud.google.com/apt/doc/apt-key.gpg |apt-key add - && \
    wget -O - https://download.docker.com/linux/debian/gpg |apt-key add - && \
    add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian $(lsb_release -cs) stable" && \
    add-apt-repository "deb [arch=amd64] https://apt.kubernetes.io/ kubernetes-xenial main"
RUN apt-get update && apt-get install -y kubelet kubeadm kubectl docker-ce docker-ce-cli containerd.io iptables
COPY 10-kubeadm.conf /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
COPY civitas.service /etc/systemd/system/civitas.service
RUN systemctl enable kubelet docker civitas
