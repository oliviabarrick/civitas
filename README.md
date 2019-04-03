civitas is an experiment at bootstrapping Kubernetes clusters where the nodes have
no prior knowledge of each other before being started, but will find each other and
come to agreement on a Kubernetes bootstrap token and the roles of each node.

# Requirements

In order for a Kubernetes cluster to be considered "civitas", it must meet the
following requirements:

* There are no permanent master IP addresses or hostnames.
* If a Kubernetes master goes away, a new one should be elected in its place.
* No Kubernetes bootstrap token should need to be preshared between nodes.
* No Kubernetes-specific keys should need to be preshared between nodes.
* Node roles are not predetermined - any node can be chosen to be a master or worker.
* Nodes must not require prior knowledge of any other nodes.
* Kubernetes cluster version upgrades must be coordinated through the cluster.
* The cluster has no external dependencies on DNS, load balancers, PKI, KV stores, S3,
  or any cloud services.
* The cluster must be idempotent, the cluster will be bootstrapped if it does not
  exist or be brought to the correct state. If it is already in a good state, no
  action should be taken.
* The cluster must be able to grow or shrink to any size.
* It must be safe to remove any node at any time.
* The Kubernetes control plane must be highly available in order to allow rotation
  of master nodes.

By meeting all of these requirements, it is possible to build clusters that require
no external infrastructure to manage, are resilient to node removal, and that are
fully decentralized.

This is useful for many use-cases, such as:

* Edge environments where external connectivity is unreliable.
* Multi-cloud environments where the cluster should be free to move between any cloud.
* Low budget clusters that cannot afford external dependencies.
* Managing many clusters where it is useful to keep the complexity contained to the
  cluster.
* Anyone looking for a low complexity deployment method for Kubernetes.

# Design

The general workflow for bootstrapping a new Kubernetes cluster is:

* Start N number of nodes.
* The node discovers the IP address of as many other Kubernetes nodes as possible.
* The node connects to each known node to advertise itself and find out about other
  nodes.
* Once enough nodes know about each other, they elect a leader which chooses M number
  of master nodes, a bootstrap token, and a certificate key for Kubeadm.
* Each node acts according to its role:
  * The first master node runs `kubeadm init`.
  * The other masters bootstrap as masters.
  * The workers bootstrap as workers.
* If a master node is unreachable for X period of time, then a worker node will be
  elected as a master which will:
  * Drain the worker.
  * Run `kubeadm reset`.
  * Run `kubeadm join` as a master node.

To accomplish this, we employ several protocols:

* [Serf](https://github.com/hashicorp/serf), by Hashicorp, an implementation of SWIM,
  for cluster membership and service discovery.
* [Raft](https://github.com/hashicorp/raft), by Hashicorp, an implementation of Raft,
  for leader election and distributed consensus.
* [Dsync](https://github.com/minio/dsync), by Minio, a distributed lock, for
  bootstrapping Raft.

Kubeadm is also used for creating Kubernetes nodes and the HA control plane.

## Peer discovery

When a cluster is first being birthed, no nodes know about each other or what their
role is. The only information they will know is the number of expected initial nodes.

Using some mechanism, each node will discover the address of other nodes. There are a
number of means to accomplish this:

* [mDNS](https://en.wikipedia.org/wiki/Multicast_DNS) can be used on a LAN.
* Cloud provider labels can be used, if the nodes have access to a cloud provider.
* [IPFS](https://github.com/ipfs/notes/issues/15) can be used to discover nodes
  that are not on the same LAN. This mechanism will be expanded on later.

Once the node has learned the IP address of some peers, it connects to them using Serf
to exchange cluster membership information. Serf is based on SWIM, a gossip protocol
for sharing information about other members in a cluster.

A pre-shared Serf encryption key is used to prevent unknown nodes from connecting.

### Peer discovery through IPFS

IPFS can be used as a DHT for storing information. This DHT is globally routable
and queriable by any node connected to IPFS. We can exploit the IPFS DHT for an
always available alternative to other means of discovering peers.

Using the method documented [here](https://github.com/ipfs/notes/issues/15), we can
pick an identifier for the Kubernetes cluster that is shared to all nodes. That
identifier is written to IPFS:

```
BLOCKNAME=$(echo -n $CLUSTER_ID | ipfs block put)
```

All nodes will write the block to IPFS, which means they will all be providers of the
block in IPFS and can be discovered:

```
ipfs dht findprovs $BLOCKNAME
```

This mechanism is slow and unreliable so is not an alternative to proper peer
discovery through Serf, but allows discovering initial node addresses.

## Generating and sharing cluster metadata

If the cluster has not yet been bootstrapped, the nodes need to agree on a number of
details:

* A bootstrap token.
* A certificate key for encrypting Kubernetes certificates for sharing between
  control plane nodes.
* The initial bootstrap master.
* Any additional master nodes.

In order to facilitate this, we will use Raft for distributed consensus. Once a Raft
quorum has been an established, a leader is elected and the leader generates the above
bootstrap information, the information is shared via the Raft replicated log to all
nodes.

After the initial bootstrap, any nodes added to the cluster will receive the Raft log
and connect as their proper role.

If a Kubernetes master is removed from Serf, then the Raft leader is responsible for
electing a new Kubernetes master and replicating that information to the other nodes.

### Leader election at bootstrap time

In order to use Raft, a leader has to be predetermined. One node bootstraps the Raft
cluster and connects to all of the others - after this, leader election proceeds as
normal. This presents an issue for civitas, since the nodes have no prior knowledge
of each other so a leader has to be elected.

To work around this, we use dsync as a distributed lock. Dsync is a gossip-based
locking protocol so it does not require an external store. All nodes request the lock
and once N/2+1 nodes have granted the lock to a node, it is elected as the initial
Raft leader.

Dsync is not intended to scale to greater than 16 nodes, so is not used beyond the
initial leader election to bootstrap Raft.

## Bootstrapping Kubernetes nodes

Once Raft has determined the node's role, bootstrap token, and certificate key, then
kubeadm configuration is written out and kubeadm is invoked.

### Bootstrapping the initial master

The initial master is responsible for generating Kubernetes certificates and
configuration. It uploads these to etcd using the new `--experimental-upload-certs`
feature.

To bootstrap an initial master, kubeadm is invoked:

```
kubeadm init --experimental-upload-certs --certificate-key $KEY --config /tmp/config.yml
```

### Bootstrapping other masters

After the cluster has been bootstrapped, other masters are joined with kubeadm:

```
kubeadm join --certificate-key $KEY --config /tmp/config.yml
```

### Bootstrapping workers

Workers are bootstrapped with kubeadm join:

```
kubeadm join --config /tmp/config.yml
```

### Changing node roles

To change a node's role, the node should be drained and then `kubeadm reset` run.

After that, the node can be bootstrapped per its role.

## Cluster upgrades

The Raft leader can coordinate a Kubernetes cluster upgrade by orchestrating the
upgrade process documented [here](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-upgrade-1-13/).
