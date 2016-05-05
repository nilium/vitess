# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: queryservice.proto

import sys
_b=sys.version_info[0]<3 and (lambda x:x) or (lambda x:x.encode('latin1'))
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from google.protobuf import reflection as _reflection
from google.protobuf import symbol_database as _symbol_database
from google.protobuf import descriptor_pb2
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


import query_pb2 as query__pb2


DESCRIPTOR = _descriptor.FileDescriptor(
  name='queryservice.proto',
  package='queryservice',
  syntax='proto3',
  serialized_pb=_b('\n\x12queryservice.proto\x12\x0cqueryservice\x1a\x0bquery.proto2\x8e\x06\n\x05Query\x12I\n\x0cGetSessionId\x12\x1a.query.GetSessionIdRequest\x1a\x1b.query.GetSessionIdResponse\"\x00\x12:\n\x07\x45xecute\x12\x15.query.ExecuteRequest\x1a\x16.query.ExecuteResponse\"\x00\x12I\n\x0c\x45xecuteBatch\x12\x1a.query.ExecuteBatchRequest\x1a\x1b.query.ExecuteBatchResponse\"\x00\x12N\n\rStreamExecute\x12\x1b.query.StreamExecuteRequest\x1a\x1c.query.StreamExecuteResponse\"\x00\x30\x01\x12\x34\n\x05\x42\x65gin\x12\x13.query.BeginRequest\x1a\x14.query.BeginResponse\"\x00\x12\x37\n\x06\x43ommit\x12\x14.query.CommitRequest\x1a\x15.query.CommitResponse\"\x00\x12=\n\x08Rollback\x12\x16.query.RollbackRequest\x1a\x17.query.RollbackResponse\"\x00\x12I\n\x0c\x42\x65ginExecute\x12\x1a.query.BeginExecuteRequest\x1a\x1b.query.BeginExecuteResponse\"\x00\x12X\n\x11\x42\x65ginExecuteBatch\x12\x1f.query.BeginExecuteBatchRequest\x1a .query.BeginExecuteBatchResponse\"\x00\x12\x43\n\nSplitQuery\x12\x18.query.SplitQueryRequest\x1a\x19.query.SplitQueryResponse\"\x00\x12K\n\x0cStreamHealth\x12\x1a.query.StreamHealthRequest\x1a\x1b.query.StreamHealthResponse\"\x00\x30\x01\x62\x06proto3')
  ,
  dependencies=[query__pb2.DESCRIPTOR,])
_sym_db.RegisterFileDescriptor(DESCRIPTOR)





import abc
from grpc.beta import implementations as beta_implementations
from grpc.framework.common import cardinality
from grpc.framework.interfaces.face import utilities as face_utilities

class BetaQueryServicer(object):
  """<fill me in later!>"""
  __metaclass__ = abc.ABCMeta
  @abc.abstractmethod
  def GetSessionId(self, request, context):
    raise NotImplementedError()
  @abc.abstractmethod
  def Execute(self, request, context):
    raise NotImplementedError()
  @abc.abstractmethod
  def ExecuteBatch(self, request, context):
    raise NotImplementedError()
  @abc.abstractmethod
  def StreamExecute(self, request, context):
    raise NotImplementedError()
  @abc.abstractmethod
  def Begin(self, request, context):
    raise NotImplementedError()
  @abc.abstractmethod
  def Commit(self, request, context):
    raise NotImplementedError()
  @abc.abstractmethod
  def Rollback(self, request, context):
    raise NotImplementedError()
  @abc.abstractmethod
  def BeginExecute(self, request, context):
    raise NotImplementedError()
  @abc.abstractmethod
  def BeginExecuteBatch(self, request, context):
    raise NotImplementedError()
  @abc.abstractmethod
  def SplitQuery(self, request, context):
    raise NotImplementedError()
  @abc.abstractmethod
  def StreamHealth(self, request, context):
    raise NotImplementedError()

class BetaQueryStub(object):
  """The interface to which stubs will conform."""
  __metaclass__ = abc.ABCMeta
  @abc.abstractmethod
  def GetSessionId(self, request, timeout):
    raise NotImplementedError()
  GetSessionId.future = None
  @abc.abstractmethod
  def Execute(self, request, timeout):
    raise NotImplementedError()
  Execute.future = None
  @abc.abstractmethod
  def ExecuteBatch(self, request, timeout):
    raise NotImplementedError()
  ExecuteBatch.future = None
  @abc.abstractmethod
  def StreamExecute(self, request, timeout):
    raise NotImplementedError()
  @abc.abstractmethod
  def Begin(self, request, timeout):
    raise NotImplementedError()
  Begin.future = None
  @abc.abstractmethod
  def Commit(self, request, timeout):
    raise NotImplementedError()
  Commit.future = None
  @abc.abstractmethod
  def Rollback(self, request, timeout):
    raise NotImplementedError()
  Rollback.future = None
  @abc.abstractmethod
  def BeginExecute(self, request, timeout):
    raise NotImplementedError()
  BeginExecute.future = None
  @abc.abstractmethod
  def BeginExecuteBatch(self, request, timeout):
    raise NotImplementedError()
  BeginExecuteBatch.future = None
  @abc.abstractmethod
  def SplitQuery(self, request, timeout):
    raise NotImplementedError()
  SplitQuery.future = None
  @abc.abstractmethod
  def StreamHealth(self, request, timeout):
    raise NotImplementedError()

def beta_create_Query_server(servicer, pool=None, pool_size=None, default_timeout=None, maximum_timeout=None):
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  request_deserializers = {
    ('queryservice.Query', 'Begin'): query_pb2.BeginRequest.FromString,
    ('queryservice.Query', 'BeginExecute'): query_pb2.BeginExecuteRequest.FromString,
    ('queryservice.Query', 'BeginExecuteBatch'): query_pb2.BeginExecuteBatchRequest.FromString,
    ('queryservice.Query', 'Commit'): query_pb2.CommitRequest.FromString,
    ('queryservice.Query', 'Execute'): query_pb2.ExecuteRequest.FromString,
    ('queryservice.Query', 'ExecuteBatch'): query_pb2.ExecuteBatchRequest.FromString,
    ('queryservice.Query', 'GetSessionId'): query_pb2.GetSessionIdRequest.FromString,
    ('queryservice.Query', 'Rollback'): query_pb2.RollbackRequest.FromString,
    ('queryservice.Query', 'SplitQuery'): query_pb2.SplitQueryRequest.FromString,
    ('queryservice.Query', 'StreamExecute'): query_pb2.StreamExecuteRequest.FromString,
    ('queryservice.Query', 'StreamHealth'): query_pb2.StreamHealthRequest.FromString,
  }
  response_serializers = {
    ('queryservice.Query', 'Begin'): query_pb2.BeginResponse.SerializeToString,
    ('queryservice.Query', 'BeginExecute'): query_pb2.BeginExecuteResponse.SerializeToString,
    ('queryservice.Query', 'BeginExecuteBatch'): query_pb2.BeginExecuteBatchResponse.SerializeToString,
    ('queryservice.Query', 'Commit'): query_pb2.CommitResponse.SerializeToString,
    ('queryservice.Query', 'Execute'): query_pb2.ExecuteResponse.SerializeToString,
    ('queryservice.Query', 'ExecuteBatch'): query_pb2.ExecuteBatchResponse.SerializeToString,
    ('queryservice.Query', 'GetSessionId'): query_pb2.GetSessionIdResponse.SerializeToString,
    ('queryservice.Query', 'Rollback'): query_pb2.RollbackResponse.SerializeToString,
    ('queryservice.Query', 'SplitQuery'): query_pb2.SplitQueryResponse.SerializeToString,
    ('queryservice.Query', 'StreamExecute'): query_pb2.StreamExecuteResponse.SerializeToString,
    ('queryservice.Query', 'StreamHealth'): query_pb2.StreamHealthResponse.SerializeToString,
  }
  method_implementations = {
    ('queryservice.Query', 'Begin'): face_utilities.unary_unary_inline(servicer.Begin),
    ('queryservice.Query', 'BeginExecute'): face_utilities.unary_unary_inline(servicer.BeginExecute),
    ('queryservice.Query', 'BeginExecuteBatch'): face_utilities.unary_unary_inline(servicer.BeginExecuteBatch),
    ('queryservice.Query', 'Commit'): face_utilities.unary_unary_inline(servicer.Commit),
    ('queryservice.Query', 'Execute'): face_utilities.unary_unary_inline(servicer.Execute),
    ('queryservice.Query', 'ExecuteBatch'): face_utilities.unary_unary_inline(servicer.ExecuteBatch),
    ('queryservice.Query', 'GetSessionId'): face_utilities.unary_unary_inline(servicer.GetSessionId),
    ('queryservice.Query', 'Rollback'): face_utilities.unary_unary_inline(servicer.Rollback),
    ('queryservice.Query', 'SplitQuery'): face_utilities.unary_unary_inline(servicer.SplitQuery),
    ('queryservice.Query', 'StreamExecute'): face_utilities.unary_stream_inline(servicer.StreamExecute),
    ('queryservice.Query', 'StreamHealth'): face_utilities.unary_stream_inline(servicer.StreamHealth),
  }
  server_options = beta_implementations.server_options(request_deserializers=request_deserializers, response_serializers=response_serializers, thread_pool=pool, thread_pool_size=pool_size, default_timeout=default_timeout, maximum_timeout=maximum_timeout)
  return beta_implementations.server(method_implementations, options=server_options)

def beta_create_Query_stub(channel, host=None, metadata_transformer=None, pool=None, pool_size=None):
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  import query_pb2
  request_serializers = {
    ('queryservice.Query', 'Begin'): query_pb2.BeginRequest.SerializeToString,
    ('queryservice.Query', 'BeginExecute'): query_pb2.BeginExecuteRequest.SerializeToString,
    ('queryservice.Query', 'BeginExecuteBatch'): query_pb2.BeginExecuteBatchRequest.SerializeToString,
    ('queryservice.Query', 'Commit'): query_pb2.CommitRequest.SerializeToString,
    ('queryservice.Query', 'Execute'): query_pb2.ExecuteRequest.SerializeToString,
    ('queryservice.Query', 'ExecuteBatch'): query_pb2.ExecuteBatchRequest.SerializeToString,
    ('queryservice.Query', 'GetSessionId'): query_pb2.GetSessionIdRequest.SerializeToString,
    ('queryservice.Query', 'Rollback'): query_pb2.RollbackRequest.SerializeToString,
    ('queryservice.Query', 'SplitQuery'): query_pb2.SplitQueryRequest.SerializeToString,
    ('queryservice.Query', 'StreamExecute'): query_pb2.StreamExecuteRequest.SerializeToString,
    ('queryservice.Query', 'StreamHealth'): query_pb2.StreamHealthRequest.SerializeToString,
  }
  response_deserializers = {
    ('queryservice.Query', 'Begin'): query_pb2.BeginResponse.FromString,
    ('queryservice.Query', 'BeginExecute'): query_pb2.BeginExecuteResponse.FromString,
    ('queryservice.Query', 'BeginExecuteBatch'): query_pb2.BeginExecuteBatchResponse.FromString,
    ('queryservice.Query', 'Commit'): query_pb2.CommitResponse.FromString,
    ('queryservice.Query', 'Execute'): query_pb2.ExecuteResponse.FromString,
    ('queryservice.Query', 'ExecuteBatch'): query_pb2.ExecuteBatchResponse.FromString,
    ('queryservice.Query', 'GetSessionId'): query_pb2.GetSessionIdResponse.FromString,
    ('queryservice.Query', 'Rollback'): query_pb2.RollbackResponse.FromString,
    ('queryservice.Query', 'SplitQuery'): query_pb2.SplitQueryResponse.FromString,
    ('queryservice.Query', 'StreamExecute'): query_pb2.StreamExecuteResponse.FromString,
    ('queryservice.Query', 'StreamHealth'): query_pb2.StreamHealthResponse.FromString,
  }
  cardinalities = {
    'Begin': cardinality.Cardinality.UNARY_UNARY,
    'BeginExecute': cardinality.Cardinality.UNARY_UNARY,
    'BeginExecuteBatch': cardinality.Cardinality.UNARY_UNARY,
    'Commit': cardinality.Cardinality.UNARY_UNARY,
    'Execute': cardinality.Cardinality.UNARY_UNARY,
    'ExecuteBatch': cardinality.Cardinality.UNARY_UNARY,
    'GetSessionId': cardinality.Cardinality.UNARY_UNARY,
    'Rollback': cardinality.Cardinality.UNARY_UNARY,
    'SplitQuery': cardinality.Cardinality.UNARY_UNARY,
    'StreamExecute': cardinality.Cardinality.UNARY_STREAM,
    'StreamHealth': cardinality.Cardinality.UNARY_STREAM,
  }
  stub_options = beta_implementations.stub_options(host=host, metadata_transformer=metadata_transformer, request_serializers=request_serializers, response_deserializers=response_deserializers, thread_pool=pool, thread_pool_size=pool_size)
  return beta_implementations.dynamic_stub(channel, 'queryservice.Query', cardinalities, options=stub_options)
# @@protoc_insertion_point(module_scope)
