1. outConnRecord 表示对外链接的IP列表维护，在链接准备建立就已经add节点信息，如果在添加期间出错，再次删除。
2. ConsLink、SyncLink为每个远程节点维护两个变量
3、 ConnectingAddrs准备链接的集合
4、PeerSyncAddress、PeerConsAddress