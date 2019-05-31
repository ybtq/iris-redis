# iris-redis

用于iris的redis插件

iris自身的redis实现，会导致redis全表扫描，当redis中数据量很大的时候，耗时会很长。
本插件修改其存储机制，每个会话只存储一个键值对，不会全表扫描。
