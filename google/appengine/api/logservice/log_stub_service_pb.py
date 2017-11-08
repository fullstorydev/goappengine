#!/usr/bin/env python
#
# Copyright 2007 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#



from google.net.proto import ProtocolBuffer
import array
import base64
import dummy_thread as thread
try:
  from google3.net.proto import _net_proto___parse__python
except ImportError:
  _net_proto___parse__python = None
import sys
try:
  __import__('google.net.rpc.python.rpc_internals_lite')
  __import__('google.net.rpc.python.pywraprpc_lite')
  rpc_internals = sys.modules.get('google.net.rpc.python.rpc_internals_lite')
  pywraprpc = sys.modules.get('google.net.rpc.python.pywraprpc_lite')
  _client_stub_base_class = rpc_internals.StubbyRPCBaseStub
except ImportError:
  _client_stub_base_class = object
try:
  __import__('google.net.rpc.python.rpcserver')
  rpcserver = sys.modules.get('google.net.rpc.python.rpcserver')
  _server_stub_base_class = rpcserver.BaseRpcServer
except ImportError:
  _server_stub_base_class = object

if hasattr(ProtocolBuffer, 'ExtendableProtocolMessage'):
  _extension_runtime = True
  _ExtendableProtocolMessage = ProtocolBuffer.ExtendableProtocolMessage
else:
  _extension_runtime = False
  _ExtendableProtocolMessage = ProtocolBuffer.ProtocolMessage

from google.appengine.api.api_base_pb import *
import google.appengine.api.api_base_pb
google_dot_apphosting_dot_api_dot_api__base__pb = __import__('google.appengine.api.api_base_pb', {}, {}, [''])
from google.appengine.api.logservice.log_service_pb import *
import google.appengine.api.logservice.log_service_pb
google_dot_apphosting_dot_api_dot_logservice_dot_log__service__pb = __import__('google.appengine.api.logservice.log_service_pb', {}, {}, [''])
class AddRequestInfoRequest(ProtocolBuffer.ProtocolMessage):
  has_request_log_ = 0
  request_log_ = None

  def __init__(self, contents=None):
    self.lazy_init_lock_ = thread.allocate_lock()
    if contents is not None: self.MergeFromString(contents)

  def request_log(self):
    if self.request_log_ is None:
      self.lazy_init_lock_.acquire()
      try:
        if self.request_log_ is None: self.request_log_ = google.appengine.api.logservice.log_service_pb.RequestLog()
      finally:
        self.lazy_init_lock_.release()
    return self.request_log_

  def mutable_request_log(self): self.has_request_log_ = 1; return self.request_log()

  def clear_request_log(self):

    if self.has_request_log_:
      self.has_request_log_ = 0;
      if self.request_log_ is not None: self.request_log_.Clear()

  def has_request_log(self): return self.has_request_log_


  def MergeFrom(self, x):
    assert x is not self
    if (x.has_request_log()): self.mutable_request_log().MergeFrom(x.request_log())

  if _net_proto___parse__python is not None:
    def _CMergeFromString(self, s):
      _net_proto___parse__python.MergeFromString(self, 'apphosting.AddRequestInfoRequest', s)

  if _net_proto___parse__python is not None:
    def _CEncode(self):
      return _net_proto___parse__python.Encode(self, 'apphosting.AddRequestInfoRequest')

  if _net_proto___parse__python is not None:
    def _CEncodePartial(self):
      return _net_proto___parse__python.EncodePartial(self, 'apphosting.AddRequestInfoRequest')

  if _net_proto___parse__python is not None:
    def _CToASCII(self, output_format):
      return _net_proto___parse__python.ToASCII(self, 'apphosting.AddRequestInfoRequest', output_format)


  if _net_proto___parse__python is not None:
    def ParseASCII(self, s):
      _net_proto___parse__python.ParseASCII(self, 'apphosting.AddRequestInfoRequest', s)


  if _net_proto___parse__python is not None:
    def ParseASCIIIgnoreUnknown(self, s):
      _net_proto___parse__python.ParseASCIIIgnoreUnknown(self, 'apphosting.AddRequestInfoRequest', s)


  def Equals(self, x):
    if x is self: return 1
    if self.has_request_log_ != x.has_request_log_: return 0
    if self.has_request_log_ and self.request_log_ != x.request_log_: return 0
    return 1

  def IsInitialized(self, debug_strs=None):
    initialized = 1
    if (self.has_request_log_ and not self.request_log_.IsInitialized(debug_strs)): initialized = 0
    return initialized

  def ByteSize(self):
    n = 0
    if (self.has_request_log_): n += 1 + self.lengthString(self.request_log_.ByteSize())
    return n

  def ByteSizePartial(self):
    n = 0
    if (self.has_request_log_): n += 1 + self.lengthString(self.request_log_.ByteSizePartial())
    return n

  def Clear(self):
    self.clear_request_log()

  def OutputUnchecked(self, out):
    if (self.has_request_log_):
      out.putVarInt32(10)
      out.putVarInt32(self.request_log_.ByteSize())
      self.request_log_.OutputUnchecked(out)

  def OutputPartial(self, out):
    if (self.has_request_log_):
      out.putVarInt32(10)
      out.putVarInt32(self.request_log_.ByteSizePartial())
      self.request_log_.OutputPartial(out)

  def TryMerge(self, d):
    while d.avail() > 0:
      tt = d.getVarInt32()
      if tt == 10:
        length = d.getVarInt32()
        tmp = ProtocolBuffer.Decoder(d.buffer(), d.pos(), d.pos() + length)
        d.skip(length)
        self.mutable_request_log().TryMerge(tmp)
        continue


      if (tt == 0): raise ProtocolBuffer.ProtocolBufferDecodeError
      d.skipData(tt)


  def __str__(self, prefix="", printElemNumber=0):
    res=""
    if self.has_request_log_:
      res+=prefix+"request_log <\n"
      res+=self.request_log_.__str__(prefix + "  ", printElemNumber)
      res+=prefix+">\n"
    return res


  def _BuildTagLookupTable(sparse, maxtag, default=None):
    return tuple([sparse.get(i, default) for i in xrange(0, 1+maxtag)])

  krequest_log = 1

  _TEXT = _BuildTagLookupTable({
    0: "ErrorCode",
    1: "request_log",
  }, 1)

  _TYPES = _BuildTagLookupTable({
    0: ProtocolBuffer.Encoder.NUMERIC,
    1: ProtocolBuffer.Encoder.STRING,
  }, 1, ProtocolBuffer.Encoder.MAX_TYPE)


  _STYLE = """"""
  _STYLE_CONTENT_TYPE = """"""
  _PROTO_DESCRIPTOR_NAME = 'apphosting.AddRequestInfoRequest'
  _SERIALIZED_DESCRIPTOR = array.array('B')
  _SERIALIZED_DESCRIPTOR.fromstring(base64.decodestring("WjBhcHBob3N0aW5nL2FwaS9sb2dzZXJ2aWNlL2xvZ19zdHViX3NlcnZpY2UucHJvdG8KIGFwcGhvc3RpbmcuQWRkUmVxdWVzdEluZm9SZXF1ZXN0ExoLcmVxdWVzdF9sb2cgASgCMAs4AUoVYXBwaG9zdGluZy5SZXF1ZXN0TG9nowGqAQVjdHlwZbIBBnByb3RvMqQBFLoBmgQKMGFwcGhvc3RpbmcvYXBpL2xvZ3NlcnZpY2UvbG9nX3N0dWJfc2VydmljZS5wcm90bxIKYXBwaG9zdGluZxodYXBwaG9zdGluZy9hcGkvYXBpX2Jhc2UucHJvdG8aK2FwcGhvc3RpbmcvYXBpL2xvZ3NlcnZpY2UvbG9nX3NlcnZpY2UucHJvdG8iRAoVQWRkUmVxdWVzdEluZm9SZXF1ZXN0EisKC3JlcXVlc3RfbG9nGAEgASgLMhYuYXBwaG9zdGluZy5SZXF1ZXN0TG9nIlEKFEFkZEFwcExvZ0xpbmVSZXF1ZXN0EiUKCGxvZ19saW5lGAEgASgLMhMuYXBwaG9zdGluZy5Mb2dMaW5lEhIKCnJlcXVlc3RfaWQYAiABKAkytAEKDkxvZ1N0dWJTZXJ2aWNlElEKDkFkZFJlcXVlc3RJbmZvEiEuYXBwaG9zdGluZy5BZGRSZXF1ZXN0SW5mb1JlcXVlc3QaGi5hcHBob3N0aW5nLmJhc2UuVm9pZFByb3RvIgASTwoNQWRkQXBwTG9nTGluZRIgLmFwcGhvc3RpbmcuQWRkQXBwTG9nTGluZVJlcXVlc3QaGi5hcHBob3N0aW5nLmJhc2UuVm9pZFByb3RvIgBCPgokY29tLmdvb2dsZS5hcHBob3N0aW5nLmFwaS5sb2dzZXJ2aWNlEAIgAigBQhBMb2dTdHViU2VydmljZVBi"))
  if _net_proto___parse__python is not None:
    _net_proto___parse__python.RegisterType(
        _SERIALIZED_DESCRIPTOR.tostring())

class AddAppLogLineRequest(ProtocolBuffer.ProtocolMessage):
  has_log_line_ = 0
  log_line_ = None
  has_request_id_ = 0
  request_id_ = ""

  def __init__(self, contents=None):
    self.lazy_init_lock_ = thread.allocate_lock()
    if contents is not None: self.MergeFromString(contents)

  def log_line(self):
    if self.log_line_ is None:
      self.lazy_init_lock_.acquire()
      try:
        if self.log_line_ is None: self.log_line_ = google.appengine.api.logservice.log_service_pb.LogLine()
      finally:
        self.lazy_init_lock_.release()
    return self.log_line_

  def mutable_log_line(self): self.has_log_line_ = 1; return self.log_line()

  def clear_log_line(self):

    if self.has_log_line_:
      self.has_log_line_ = 0;
      if self.log_line_ is not None: self.log_line_.Clear()

  def has_log_line(self): return self.has_log_line_

  def request_id(self): return self.request_id_

  def set_request_id(self, x):
    self.has_request_id_ = 1
    self.request_id_ = x

  def clear_request_id(self):
    if self.has_request_id_:
      self.has_request_id_ = 0
      self.request_id_ = ""

  def has_request_id(self): return self.has_request_id_


  def MergeFrom(self, x):
    assert x is not self
    if (x.has_log_line()): self.mutable_log_line().MergeFrom(x.log_line())
    if (x.has_request_id()): self.set_request_id(x.request_id())

  if _net_proto___parse__python is not None:
    def _CMergeFromString(self, s):
      _net_proto___parse__python.MergeFromString(self, 'apphosting.AddAppLogLineRequest', s)

  if _net_proto___parse__python is not None:
    def _CEncode(self):
      return _net_proto___parse__python.Encode(self, 'apphosting.AddAppLogLineRequest')

  if _net_proto___parse__python is not None:
    def _CEncodePartial(self):
      return _net_proto___parse__python.EncodePartial(self, 'apphosting.AddAppLogLineRequest')

  if _net_proto___parse__python is not None:
    def _CToASCII(self, output_format):
      return _net_proto___parse__python.ToASCII(self, 'apphosting.AddAppLogLineRequest', output_format)


  if _net_proto___parse__python is not None:
    def ParseASCII(self, s):
      _net_proto___parse__python.ParseASCII(self, 'apphosting.AddAppLogLineRequest', s)


  if _net_proto___parse__python is not None:
    def ParseASCIIIgnoreUnknown(self, s):
      _net_proto___parse__python.ParseASCIIIgnoreUnknown(self, 'apphosting.AddAppLogLineRequest', s)


  def Equals(self, x):
    if x is self: return 1
    if self.has_log_line_ != x.has_log_line_: return 0
    if self.has_log_line_ and self.log_line_ != x.log_line_: return 0
    if self.has_request_id_ != x.has_request_id_: return 0
    if self.has_request_id_ and self.request_id_ != x.request_id_: return 0
    return 1

  def IsInitialized(self, debug_strs=None):
    initialized = 1
    if (self.has_log_line_ and not self.log_line_.IsInitialized(debug_strs)): initialized = 0
    return initialized

  def ByteSize(self):
    n = 0
    if (self.has_log_line_): n += 1 + self.lengthString(self.log_line_.ByteSize())
    if (self.has_request_id_): n += 1 + self.lengthString(len(self.request_id_))
    return n

  def ByteSizePartial(self):
    n = 0
    if (self.has_log_line_): n += 1 + self.lengthString(self.log_line_.ByteSizePartial())
    if (self.has_request_id_): n += 1 + self.lengthString(len(self.request_id_))
    return n

  def Clear(self):
    self.clear_log_line()
    self.clear_request_id()

  def OutputUnchecked(self, out):
    if (self.has_log_line_):
      out.putVarInt32(10)
      out.putVarInt32(self.log_line_.ByteSize())
      self.log_line_.OutputUnchecked(out)
    if (self.has_request_id_):
      out.putVarInt32(18)
      out.putPrefixedString(self.request_id_)

  def OutputPartial(self, out):
    if (self.has_log_line_):
      out.putVarInt32(10)
      out.putVarInt32(self.log_line_.ByteSizePartial())
      self.log_line_.OutputPartial(out)
    if (self.has_request_id_):
      out.putVarInt32(18)
      out.putPrefixedString(self.request_id_)

  def TryMerge(self, d):
    while d.avail() > 0:
      tt = d.getVarInt32()
      if tt == 10:
        length = d.getVarInt32()
        tmp = ProtocolBuffer.Decoder(d.buffer(), d.pos(), d.pos() + length)
        d.skip(length)
        self.mutable_log_line().TryMerge(tmp)
        continue
      if tt == 18:
        self.set_request_id(d.getPrefixedString())
        continue


      if (tt == 0): raise ProtocolBuffer.ProtocolBufferDecodeError
      d.skipData(tt)


  def __str__(self, prefix="", printElemNumber=0):
    res=""
    if self.has_log_line_:
      res+=prefix+"log_line <\n"
      res+=self.log_line_.__str__(prefix + "  ", printElemNumber)
      res+=prefix+">\n"
    if self.has_request_id_: res+=prefix+("request_id: %s\n" % self.DebugFormatString(self.request_id_))
    return res


  def _BuildTagLookupTable(sparse, maxtag, default=None):
    return tuple([sparse.get(i, default) for i in xrange(0, 1+maxtag)])

  klog_line = 1
  krequest_id = 2

  _TEXT = _BuildTagLookupTable({
    0: "ErrorCode",
    1: "log_line",
    2: "request_id",
  }, 2)

  _TYPES = _BuildTagLookupTable({
    0: ProtocolBuffer.Encoder.NUMERIC,
    1: ProtocolBuffer.Encoder.STRING,
    2: ProtocolBuffer.Encoder.STRING,
  }, 2, ProtocolBuffer.Encoder.MAX_TYPE)


  _STYLE = """"""
  _STYLE_CONTENT_TYPE = """"""
  _PROTO_DESCRIPTOR_NAME = 'apphosting.AddAppLogLineRequest'
  _SERIALIZED_DESCRIPTOR = array.array('B')
  _SERIALIZED_DESCRIPTOR.fromstring(base64.decodestring("WjBhcHBob3N0aW5nL2FwaS9sb2dzZXJ2aWNlL2xvZ19zdHViX3NlcnZpY2UucHJvdG8KH2FwcGhvc3RpbmcuQWRkQXBwTG9nTGluZVJlcXVlc3QTGghsb2dfbGluZSABKAIwCzgBShJhcHBob3N0aW5nLkxvZ0xpbmWjAaoBBWN0eXBlsgEGcHJvdG8ypAEUExoKcmVxdWVzdF9pZCACKAIwCTgBFMIBIGFwcGhvc3RpbmcuQWRkUmVxdWVzdEluZm9SZXF1ZXN0"))
  if _net_proto___parse__python is not None:
    _net_proto___parse__python.RegisterType(
        _SERIALIZED_DESCRIPTOR.tostring())



class _LogStubService_ClientBaseStub(_client_stub_base_class):
  """Makes Stubby RPC calls to a LogStubService server."""

  __slots__ = (
      '_protorpc_AddRequestInfo', '_full_name_AddRequestInfo',
      '_protorpc_AddAppLogLine', '_full_name_AddAppLogLine',
  )

  def __init__(self, rpc_stub, rpc_factory=None):
    super(_LogStubService_ClientBaseStub, self).__init__(
        None, inject_stub=rpc_stub, rpc_factory=rpc_factory)

    self._protorpc_AddRequestInfo = pywraprpc.RPC()
    self._full_name_AddRequestInfo = self._stub.GetFullMethodName(
        'AddRequestInfo')

    self._protorpc_AddAppLogLine = pywraprpc.RPC()
    self._full_name_AddAppLogLine = self._stub.GetFullMethodName(
        'AddAppLogLine')

  def AddRequestInfo(self, request, rpc=None, callback=None, response=None):
    """Make a AddRequestInfo RPC call.

    Args:
      request: a AddRequestInfoRequest instance.
      rpc: Optional RPC instance to use for the call.
      callback: Optional final callback. Will be called as
          callback(rpc, result) when the rpc completes. If None, the
          call is synchronous.
      response: Optional ProtocolMessage to be filled in with response.

    Returns:
      The google_dot_apphosting_dot_api_dot_api__base__pb.VoidProto if callback is None. Otherwise, returns None.
    """

    if response is None:
      response = google_dot_apphosting_dot_api_dot_api__base__pb.VoidProto
    return self._MakeCall(rpc,
                          self._full_name_AddRequestInfo,
                          'AddRequestInfo',
                          request,
                          response,
                          callback,
                          self._protorpc_AddRequestInfo,
                          package_name='apphosting')

  def AddAppLogLine(self, request, rpc=None, callback=None, response=None):
    """Make a AddAppLogLine RPC call.

    Args:
      request: a AddAppLogLineRequest instance.
      rpc: Optional RPC instance to use for the call.
      callback: Optional final callback. Will be called as
          callback(rpc, result) when the rpc completes. If None, the
          call is synchronous.
      response: Optional ProtocolMessage to be filled in with response.

    Returns:
      The google_dot_apphosting_dot_api_dot_api__base__pb.VoidProto if callback is None. Otherwise, returns None.
    """

    if response is None:
      response = google_dot_apphosting_dot_api_dot_api__base__pb.VoidProto
    return self._MakeCall(rpc,
                          self._full_name_AddAppLogLine,
                          'AddAppLogLine',
                          request,
                          response,
                          callback,
                          self._protorpc_AddAppLogLine,
                          package_name='apphosting')


class _LogStubService_ClientStub(_LogStubService_ClientBaseStub):
  __slots__ = ('_params',)
  def __init__(self, rpc_stub_parameters, service_name, rpc_factory=None):
    if service_name is None:
      service_name = 'LogStubService'
    stub = pywraprpc.RPC_GenericStub(service_name, rpc_stub_parameters)
    super(_LogStubService_ClientStub, self).__init__(stub, rpc_factory=rpc_factory)
    self._params = rpc_stub_parameters


class _LogStubService_RPC2ClientStub(_LogStubService_ClientBaseStub):
  __slots__ = ()
  def __init__(self, server, channel, service_name, rpc_factory=None):
    if service_name is None:
      service_name = 'LogStubService'
    if channel is None:
      if server is None:
        raise RuntimeError('Invalid argument combination to create a stub')
      channel = pywraprpc.NewClientChannel(server)
    elif channel.version() == 1:
      raise RuntimeError('Expecting an RPC2 channel to create the stub')
    stub = pywraprpc.RPC_GenericStub(service_name, channel)
    super(_LogStubService_RPC2ClientStub, self).__init__(stub, rpc_factory=rpc_factory)


class LogStubService(_server_stub_base_class):
  """Base class for LogStubService Stubby servers."""

  @classmethod
  def _MethodSignatures(cls):
    """Returns a dict of {<method-name>: (<request-type>, <response-type>)}."""
    return {
      'AddRequestInfo': (AddRequestInfoRequest, google_dot_apphosting_dot_api_dot_api__base__pb.VoidProto),
      'AddAppLogLine': (AddAppLogLineRequest, google_dot_apphosting_dot_api_dot_api__base__pb.VoidProto),
      }

  @classmethod
  def _StreamMethodSignatures(cls):
    """Returns a dict of {<method-name>: (<request-type>, <stream-type>, <response-type>)}."""
    return {
      }

  def __init__(self, *args, **kwargs):
    """Creates a Stubby RPC server.

    The arguments to this constructor are the same as the arguments to
    BaseRpcServer.__init__ in rpcserver.py *MINUS* export_name. This
    constructor passes its own value for export_name to
    BaseRpcServer.__init__, so callers of this constructor should only
    pass to this constructor values corresponding to
    BaseRpcServer.__init__'s remaining arguments.
    """
    if _server_stub_base_class is object:
      raise NotImplementedError('Add //net/rpc/python:rpcserver as a '
                                'dependency for Stubby server support.')
    _server_stub_base_class.__init__(self, 'apphosting.LogStubService', *args, **kwargs)

  @staticmethod
  def NewStub(rpc_stub_parameters, service_name=None, rpc_factory=None):
    """Creates a new LogStubService Stubby client stub.

    Args:
      rpc_stub_parameters: an RPC_StubParameters instance.
      service_name: the service name used by the Stubby server.
      rpc_factory: the rpc factory to use if no rpc argument is specified.
    """

    if _client_stub_base_class is object:
      raise RuntimeError('Add //net/rpc/python as a dependency to use Stubby')
    return _LogStubService_ClientStub(
        rpc_stub_parameters, service_name, rpc_factory=rpc_factory)

  @staticmethod
  def NewRPC2Stub(
      server=None, channel=None, service_name=None, rpc_factory=None):
    """Creates a new LogStubService Stubby2 client stub.

    Args:
      server: host:port or bns address.
      channel: directly use a channel to create a stub. Will ignore server
          argument if this is specified.
      service_name: the service name used by the Stubby server.
      rpc_factory: the rpc factory to use if no rpc argument is specified.
    """

    if _client_stub_base_class is object:
      raise RuntimeError('Add //net/rpc/python as a dependency to use Stubby')
    return _LogStubService_RPC2ClientStub(
        server, channel, service_name, rpc_factory=rpc_factory)

  def AddRequestInfo(self, rpc, request, response):
    """Handles a AddRequestInfo RPC call. You should override this.

    Args:
      rpc: a Stubby RPC object
      request: a AddRequestInfoRequest that contains the client request
      response: a google_dot_apphosting_dot_api_dot_api__base__pb.VoidProto that should be modified to send the response
    """
    raise NotImplementedError


  def AddAppLogLine(self, rpc, request, response):
    """Handles a AddAppLogLine RPC call. You should override this.

    Args:
      rpc: a Stubby RPC object
      request: a AddAppLogLineRequest that contains the client request
      response: a google_dot_apphosting_dot_api_dot_api__base__pb.VoidProto that should be modified to send the response
    """
    raise NotImplementedError

  def _AddMethodAttributes(self):
    """Sets attributes on Python RPC handlers.

    See BaseRpcServer in rpcserver.py for details.
    """
    rpcserver._GetHandlerDecorator(
        getattr(self.AddRequestInfo, '__func__'),
        AddRequestInfoRequest,
        google_dot_apphosting_dot_api_dot_api__base__pb.VoidProto,
        None,
        'INTEGRITY')
    rpcserver._GetHandlerDecorator(
        getattr(self.AddAppLogLine, '__func__'),
        AddAppLogLineRequest,
        google_dot_apphosting_dot_api_dot_api__base__pb.VoidProto,
        None,
        'INTEGRITY')

if _extension_runtime:
  pass

__all__ = ['AddRequestInfoRequest','AddAppLogLineRequest','LogStubService']
