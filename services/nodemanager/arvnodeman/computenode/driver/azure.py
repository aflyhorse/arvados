#!/usr/bin/env python

from __future__ import absolute_import, print_function

import time

import libcloud.compute.base as cloud_base
import libcloud.compute.providers as cloud_provider
import libcloud.compute.types as cloud_types
from libcloud.common.exceptions import BaseHTTPError

from . import BaseComputeNodeDriver
from .. import arvados_node_fqdn, arvados_timestamp, ARVADOS_TIMEFMT

class ComputeNodeDriver(BaseComputeNodeDriver):

    DEFAULT_DRIVER = cloud_provider.get_driver(cloud_types.Provider.AZURE_ARM)
    SEARCH_CACHE = {}
    CLOUD_ERRORS = BaseComputeNodeDriver.CLOUD_ERRORS + (BaseHTTPError,)

    def __init__(self, auth_kwargs, list_kwargs, create_kwargs,
                 driver_class=DEFAULT_DRIVER):

        if not list_kwargs.get("ex_resource_group"):
            raise Exception("Must include ex_resource_group in Cloud List configuration (list_kwargs)")

        create_kwargs["ex_resource_group"] = list_kwargs["ex_resource_group"]

        self.tags = {key[4:]: value
                     for key, value in create_kwargs.iteritems()
                     if key.startswith('tag_')}
        # filter out tags from create_kwargs
        create_kwargs = {key: value
                         for key, value in create_kwargs.iteritems()
                         if not key.startswith('tag_')}
        super(ComputeNodeDriver, self).__init__(
            auth_kwargs, list_kwargs, create_kwargs,
            driver_class)

    def arvados_create_kwargs(self, arvados_node):
        cluster_id, _, node_id = arvados_node['uuid'].split('-')
        name = 'compute-{}-{}'.format(node_id, cluster_id)
        tags = {
            'booted_at': time.strftime(ARVADOS_TIMEFMT, time.gmtime()),
            'arv-ping-url': self._make_ping_url(arvados_node)
        }
        tags.update(self.tags)
        return {
            'name': name,
            'ex_tags': tags,
        }

    def sync_node(self, cloud_node, arvados_node):
        try:
            self.real.ex_create_tags(cloud_node,
                                     {'hostname': arvados_node_fqdn(arvados_node)})
            return True
        except BaseHTTPError as b:
            return False

    def _init_image(self, urn):
        return "image", self.get_image(urn)

    def post_create_node(self, cloud_node):
        self.real.ex_run_command(cloud_node,
                                 """bash -c '
                                 mkdir -p /var/tmp/arv-node-data/meta-data
                                 echo "%s" > /var/tmp/arv-node-data/arv-ping-url
                                 echo "%s" > /var/tmp/arv-node-data/meta-data/instance-id
                                 echo "%s" > /var/tmp/arv-node-data/meta-data/instance-type
                                 echo "%s" > /var/tmp/arv-node-data/meta-data/local-ipv4
                                 '""" % (cloud_node.extra["tags"]["arv-ping-url"],
                                         cloud_node.id,
                                         cloud_node.extra["properties"]["hardwareProfile"]["vmSize"],
                                         cloud_node.private_ips[0]),
                                 timestamp=int(time.time()))

    def list_nodes(self):
        # Azure only supports filtering node lists by resource group.
        # Do our own filtering based on tag.
        nodes = [node for node in
                super(ComputeNodeDriver, self).list_nodes()
                if node.extra["tags"].get("arvados-class") == self.tags["arvados-class"]]
        for n in nodes:
            # Need to populate Node.size
            if not n.size:
                n.size = self.sizes[n.extra["properties"]["hardwareProfile"]["vmSize"]]
        return nodes

    def broken(self, cloud_node):
        """Return true if libcloud has indicated the node is in a "broken" state."""
        # UNKNOWN means the node state is unrecognized, which in practice means some combination
        # of failure that the Azure libcloud driver doesn't know how to interpret.
        return (cloud_node.state in (cloud_types.NodeState.ERROR, cloud_types.NodeState.UNKNOWN))

    @classmethod
    def node_fqdn(cls, node):
        return node.extra["tags"].get("hostname")

    @classmethod
    def node_start_time(cls, node):
        return arvados_timestamp(node.extra["tags"].get("booted_at"))
