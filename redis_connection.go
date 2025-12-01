package flam

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConnection interface {
	ACLDryRun(ctx context.Context, username string, command ...interface{}) *redis.StringCmd
	ACLLog(ctx context.Context, count int64) *redis.ACLLogCmd
	ACLLogReset(ctx context.Context) *redis.StatusCmd
	AddHook(hook redis.Hook)
	Append(ctx context.Context, key, value string) *redis.IntCmd
	BFAdd(ctx context.Context, key string, element interface{}) *redis.BoolCmd
	BFCard(ctx context.Context, key string) *redis.IntCmd
	BFExists(ctx context.Context, key string, element interface{}) *redis.BoolCmd
	BFInfo(ctx context.Context, key string) *redis.BFInfoCmd
	BFInfoArg(ctx context.Context, key, option string) *redis.BFInfoCmd
	BFInfoCapacity(ctx context.Context, key string) *redis.BFInfoCmd
	BFInfoExpansion(ctx context.Context, key string) *redis.BFInfoCmd
	BFInfoFilters(ctx context.Context, key string) *redis.BFInfoCmd
	BFInfoItems(ctx context.Context, key string) *redis.BFInfoCmd
	BFInfoSize(ctx context.Context, key string) *redis.BFInfoCmd
	BFInsert(ctx context.Context, key string, options *redis.BFInsertOptions, elements ...interface{}) *redis.BoolSliceCmd
	BFLoadChunk(ctx context.Context, key string, iterator int64, data interface{}) *redis.StatusCmd
	BFMAdd(ctx context.Context, key string, elements ...interface{}) *redis.BoolSliceCmd
	BFMExists(ctx context.Context, key string, elements ...interface{}) *redis.BoolSliceCmd
	BFReserve(ctx context.Context, key string, errorRate float64, capacity int64) *redis.StatusCmd
	BFReserveExpansion(ctx context.Context, key string, errorRate float64, capacity, expansion int64) *redis.StatusCmd
	BFReserveNonScaling(ctx context.Context, key string, errorRate float64, capacity int64) *redis.StatusCmd
	BFReserveWithArgs(ctx context.Context, key string, options *redis.BFReserveOptions) *redis.StatusCmd
	BFScanDump(ctx context.Context, key string, iterator int64) *redis.ScanDumpCmd
	BLMPop(ctx context.Context, timeout time.Duration, direction string, count int64, keys ...string) *redis.KeyValuesCmd
	BLMove(ctx context.Context, source, destination, srcpos, destpos string, imeout time.Duration) *redis.StringCmd
	BLPop(ctx context.Context, timeout time.Duration, keys ...string) *redis.StringSliceCmd
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) *redis.StringSliceCmd
	BRPopLPush(ctx context.Context, source, destination string, timeout time.Duration) *redis.StringCmd
	BZMPop(ctx context.Context, timeout time.Duration, order string, count int64, keys ...string) *redis.ZSliceWithKeyCmd
	BZPopMax(ctx context.Context, timeout time.Duration, keys ...string) *redis.ZWithKeyCmd
	BZPopMin(ctx context.Context, timeout time.Duration, keys ...string) *redis.ZWithKeyCmd
	BgRewriteAOF(ctx context.Context) *redis.StatusCmd
	BgSave(ctx context.Context) *redis.StatusCmd
	BitCount(ctx context.Context, key string, bitCount *redis.BitCount) *redis.IntCmd
	BitField(ctx context.Context, key string, values ...interface{}) *redis.IntSliceCmd
	BitFieldRO(ctx context.Context, key string, values ...interface{}) *redis.IntSliceCmd
	BitOpAnd(ctx context.Context, destKey string, keys ...string) *redis.IntCmd
	BitOpNot(ctx context.Context, destKey string, key string) *redis.IntCmd
	BitOpOr(ctx context.Context, destKey string, keys ...string) *redis.IntCmd
	BitOpXor(ctx context.Context, destKey string, keys ...string) *redis.IntCmd
	BitPos(ctx context.Context, key string, bit int64, pos ...int64) *redis.IntCmd
	BitPosSpan(ctx context.Context, key string, bit int8, start, end int64, span string) *redis.IntCmd
	CFAdd(ctx context.Context, key string, element interface{}) *redis.BoolCmd
	CFAddNX(ctx context.Context, key string, element interface{}) *redis.BoolCmd
	CFCount(ctx context.Context, key string, element interface{}) *redis.IntCmd
	CFDel(ctx context.Context, key string, element interface{}) *redis.BoolCmd
	CFExists(ctx context.Context, key string, element interface{}) *redis.BoolCmd
	CFInfo(ctx context.Context, key string) *redis.CFInfoCmd
	CFInsert(ctx context.Context, key string, options *redis.CFInsertOptions, elements ...interface{}) *redis.BoolSliceCmd
	CFInsertNX(ctx context.Context, key string, options *redis.CFInsertOptions, elements ...interface{}) *redis.IntSliceCmd
	CFLoadChunk(ctx context.Context, key string, iterator int64, data interface{}) *redis.StatusCmd
	CFMExists(ctx context.Context, key string, elements ...interface{}) *redis.BoolSliceCmd
	CFReserve(ctx context.Context, key string, capacity int64) *redis.StatusCmd
	CFReserveBucketSize(ctx context.Context, key string, capacity int64, bucketsize int64) *redis.StatusCmd
	CFReserveExpansion(ctx context.Context, key string, capacity int64, expansion int64) *redis.StatusCmd
	CFReserveMaxIterations(ctx context.Context, key string, capacity int64, maxiterations int64) *redis.StatusCmd
	CFReserveWithArgs(ctx context.Context, key string, options *redis.CFReserveOptions) *redis.StatusCmd
	CFScanDump(ctx context.Context, key string, iterator int64) *redis.ScanDumpCmd
	CMSIncrBy(ctx context.Context, key string, elements ...interface{}) *redis.IntSliceCmd
	CMSInfo(ctx context.Context, key string) *redis.CMSInfoCmd
	CMSInitByDim(ctx context.Context, key string, width, depth int64) *redis.StatusCmd
	CMSInitByProb(ctx context.Context, key string, errorRate, probability float64) *redis.StatusCmd
	CMSMerge(ctx context.Context, destKey string, sourceKeys ...string) *redis.StatusCmd
	CMSMergeWithWeight(ctx context.Context, destKey string, sourceKeys map[string]int64) *redis.StatusCmd
	CMSQuery(ctx context.Context, key string, elements ...interface{}) *redis.IntSliceCmd
	ClientGetName(ctx context.Context) *redis.StringCmd
	ClientID(ctx context.Context) *redis.IntCmd
	ClientInfo(ctx context.Context) *redis.ClientInfoCmd
	ClientKill(ctx context.Context, ipPort string) *redis.StatusCmd
	ClientKillByFilter(ctx context.Context, keys ...string) *redis.IntCmd
	ClientList(ctx context.Context) *redis.StringCmd
	ClientPause(ctx context.Context, dur time.Duration) *redis.BoolCmd
	ClientUnblock(ctx context.Context, id int64) *redis.IntCmd
	ClientUnblockWithError(ctx context.Context, id int64) *redis.IntCmd
	ClientUnpause(ctx context.Context) *redis.BoolCmd
	Close() error
	ClusterAddSlots(ctx context.Context, slots ...int) *redis.StatusCmd
	ClusterAddSlotsRange(ctx context.Context, min, max int) *redis.StatusCmd
	ClusterCountFailureReports(ctx context.Context, nodeID string) *redis.IntCmd
	ClusterCountKeysInSlot(ctx context.Context, slot int) *redis.IntCmd
	ClusterDelSlots(ctx context.Context, slots ...int) *redis.StatusCmd
	ClusterDelSlotsRange(ctx context.Context, min, max int) *redis.StatusCmd
	ClusterFailover(ctx context.Context) *redis.StatusCmd
	ClusterForget(ctx context.Context, nodeID string) *redis.StatusCmd
	ClusterGetKeysInSlot(ctx context.Context, slot int, count int) *redis.StringSliceCmd
	ClusterInfo(ctx context.Context) *redis.StringCmd
	ClusterKeySlot(ctx context.Context, key string) *redis.IntCmd
	ClusterLinks(ctx context.Context) *redis.ClusterLinksCmd
	ClusterMeet(ctx context.Context, host, port string) *redis.StatusCmd
	ClusterMyShardID(ctx context.Context) *redis.StringCmd
	ClusterNodes(ctx context.Context) *redis.StringCmd
	ClusterReplicate(ctx context.Context, nodeID string) *redis.StatusCmd
	ClusterResetHard(ctx context.Context) *redis.StatusCmd
	ClusterResetSoft(ctx context.Context) *redis.StatusCmd
	ClusterSaveConfig(ctx context.Context) *redis.StatusCmd
	ClusterShards(ctx context.Context) *redis.ClusterShardsCmd
	ClusterSlaves(ctx context.Context, nodeID string) *redis.StringSliceCmd
	ClusterSlots(ctx context.Context) *redis.ClusterSlotsCmd
	Command(ctx context.Context) *redis.CommandsInfoCmd
	CommandGetKeys(ctx context.Context, commands ...interface{}) *redis.StringSliceCmd
	CommandGetKeysAndFlags(ctx context.Context, commands ...interface{}) *redis.KeyFlagsCmd
	CommandList(ctx context.Context, filter *redis.FilterBy) *redis.StringSliceCmd
	ConfigGet(ctx context.Context, parameter string) *redis.MapStringStringCmd
	ConfigResetStat(ctx context.Context) *redis.StatusCmd
	ConfigRewrite(ctx context.Context) *redis.StatusCmd
	ConfigSet(ctx context.Context, parameter, value string) *redis.StatusCmd
	Conn() *redis.Conn
	Copy(ctx context.Context, sourceKey, destKey string, db int, replace bool) *redis.IntCmd
	DBSize(ctx context.Context) *redis.IntCmd
	DebugObject(ctx context.Context, key string) *redis.StringCmd
	Decr(ctx context.Context, key string) *redis.IntCmd
	DecrBy(ctx context.Context, key string, decrement int64) *redis.IntCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Do(ctx context.Context, args ...interface{}) *redis.Cmd
	Dump(ctx context.Context, key string) *redis.StringCmd
	Echo(ctx context.Context, message interface{}) *redis.StringCmd
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	EvalRO(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) *redis.Cmd
	EvalShaRO(ctx context.Context, sha1 string, keys []string, args ...interface{}) *redis.Cmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	ExpireAt(ctx context.Context, key string, tm time.Time) *redis.BoolCmd
	ExpireGT(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	ExpireLT(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	ExpireNX(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	ExpireTime(ctx context.Context, key string) *redis.DurationCmd
	ExpireXX(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	FCall(ctx context.Context, function string, keys []string, args ...interface{}) *redis.Cmd
	FCallRO(ctx context.Context, function string, keys []string, args ...interface{}) *redis.Cmd
	FCallRo(ctx context.Context, function string, keys []string, args ...interface{}) *redis.Cmd
	FTAggregate(ctx context.Context, index string, query string) *redis.MapStringInterfaceCmd
	FTAggregateWithArgs(ctx context.Context, index string, query string, options *redis.FTAggregateOptions) *redis.AggregateCmd
	FTAliasAdd(ctx context.Context, index string, alias string) *redis.StatusCmd
	FTAliasDel(ctx context.Context, alias string) *redis.StatusCmd
	FTAliasUpdate(ctx context.Context, index string, alias string) *redis.StatusCmd
	FTAlter(ctx context.Context, index string, skipInitialScan bool, definition []interface{}) *redis.StatusCmd
	FTConfigGet(ctx context.Context, option string) *redis.MapMapStringInterfaceCmd
	FTConfigSet(ctx context.Context, option string, value interface{}) *redis.StatusCmd
	FTCreate(ctx context.Context, index string, options *redis.FTCreateOptions, schema ...*redis.FieldSchema) *redis.StatusCmd
	FTCursorDel(ctx context.Context, index string, cursorId int) *redis.StatusCmd
	FTCursorRead(ctx context.Context, index string, cursorId int, count int) *redis.MapStringInterfaceCmd
	FTDictAdd(ctx context.Context, dict string, term ...interface{}) *redis.IntCmd
	FTDictDel(ctx context.Context, dict string, term ...interface{}) *redis.IntCmd
	FTDictDump(ctx context.Context, dict string) *redis.StringSliceCmd
	FTDropIndex(ctx context.Context, index string) *redis.StatusCmd
	FTDropIndexWithArgs(ctx context.Context, index string, options *redis.FTDropIndexOptions) *redis.StatusCmd
	FTExplain(ctx context.Context, index string, query string) *redis.StringCmd
	FTExplainCli(ctx context.Context, key, path string) error
	FTExplainWithArgs(ctx context.Context, index string, query string, options *redis.FTExplainOptions) *redis.StringCmd
	FTInfo(ctx context.Context, index string) *redis.FTInfoCmd
	FTSearch(ctx context.Context, index string, query string) *redis.FTSearchCmd
	FTSearchWithArgs(ctx context.Context, index string, query string, options *redis.FTSearchOptions) *redis.FTSearchCmd
	FTSpellCheck(ctx context.Context, index string, query string) *redis.FTSpellCheckCmd
	FTSpellCheckWithArgs(ctx context.Context, index string, query string, options *redis.FTSpellCheckOptions) *redis.FTSpellCheckCmd
	FTSynDump(ctx context.Context, index string) *redis.FTSynDumpCmd
	FTSynUpdate(ctx context.Context, index string, synGroupId interface{}, terms []interface{}) *redis.StatusCmd
	FTSynUpdateWithArgs(ctx context.Context, index string, synGroupId interface{}, options *redis.FTSynUpdateOptions, terms []interface{}) *redis.StatusCmd
	FTTagVals(ctx context.Context, index string, field string) *redis.StringSliceCmd
	FT_List(ctx context.Context) *redis.StringSliceCmd
	FlushAll(ctx context.Context) *redis.StatusCmd
	FlushAllAsync(ctx context.Context) *redis.StatusCmd
	FlushDB(ctx context.Context) *redis.StatusCmd
	FlushDBAsync(ctx context.Context) *redis.StatusCmd
	FunctionDelete(ctx context.Context, libName string) *redis.StringCmd
	FunctionDump(ctx context.Context) *redis.StringCmd
	FunctionFlush(ctx context.Context) *redis.StringCmd
	FunctionFlushAsync(ctx context.Context) *redis.StringCmd
	FunctionKill(ctx context.Context) *redis.StringCmd
	FunctionList(ctx context.Context, q redis.FunctionListQuery) *redis.FunctionListCmd
	FunctionLoad(ctx context.Context, code string) *redis.StringCmd
	FunctionLoadReplace(ctx context.Context, code string) *redis.StringCmd
	FunctionRestore(ctx context.Context, libDump string) *redis.StringCmd
	FunctionStats(ctx context.Context) *redis.FunctionStatsCmd
	GeoAdd(ctx context.Context, key string, geoLocation ...*redis.GeoLocation) *redis.IntCmd
	GeoDist(ctx context.Context, key string, member1, member2, unit string) *redis.FloatCmd
	GeoHash(ctx context.Context, key string, members ...string) *redis.StringSliceCmd
	GeoPos(ctx context.Context, key string, members ...string) *redis.GeoPosCmd
	GeoRadius(ctx context.Context, key string, longitude, latitude float64, query *redis.GeoRadiusQuery) *redis.GeoLocationCmd
	GeoRadiusByMember(ctx context.Context, key, member string, query *redis.GeoRadiusQuery) *redis.GeoLocationCmd
	GeoRadiusByMemberStore(ctx context.Context, key, member string, query *redis.GeoRadiusQuery) *redis.IntCmd
	GeoRadiusStore(ctx context.Context, key string, longitude, latitude float64, query *redis.GeoRadiusQuery) *redis.IntCmd
	GeoSearch(ctx context.Context, key string, q *redis.GeoSearchQuery) *redis.StringSliceCmd
	GeoSearchLocation(ctx context.Context, key string, q *redis.GeoSearchLocationQuery) *redis.GeoSearchLocationCmd
	GeoSearchStore(ctx context.Context, key, store string, q *redis.GeoSearchStoreQuery) *redis.IntCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	GetBit(ctx context.Context, key string, offset int64) *redis.IntCmd
	GetDel(ctx context.Context, key string) *redis.StringCmd
	GetEx(ctx context.Context, key string, expiration time.Duration) *redis.StringCmd
	GetRange(ctx context.Context, key string, start, end int64) *redis.StringCmd
	GetSet(ctx context.Context, key string, value interface{}) *redis.StringCmd
	HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd
	HExists(ctx context.Context, key, field string) *redis.BoolCmd
	HExpire(ctx context.Context, key string, expiration time.Duration, fields ...string) *redis.IntSliceCmd
	HExpireAt(ctx context.Context, key string, tm time.Time, fields ...string) *redis.IntSliceCmd
	HExpireAtWithArgs(ctx context.Context, key string, tm time.Time, expirationArgs redis.HExpireArgs, fields ...string) *redis.IntSliceCmd
	HExpireTime(ctx context.Context, key string, fields ...string) *redis.IntSliceCmd
	HExpireWithArgs(ctx context.Context, key string, expiration time.Duration, expirationArgs redis.HExpireArgs, fields ...string) *redis.IntSliceCmd
	HGet(ctx context.Context, key, field string) *redis.StringCmd
	HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd
	HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd
	HIncrByFloat(ctx context.Context, key, field string, incr float64) *redis.FloatCmd
	HKeys(ctx context.Context, key string) *redis.StringSliceCmd
	HLen(ctx context.Context, key string) *redis.IntCmd
	HMGet(ctx context.Context, key string, fields ...string) *redis.SliceCmd
	HMSet(ctx context.Context, key string, values ...interface{}) *redis.BoolCmd
	HPExpire(ctx context.Context, key string, expiration time.Duration, fields ...string) *redis.IntSliceCmd
	HPExpireAt(ctx context.Context, key string, tm time.Time, fields ...string) *redis.IntSliceCmd
	HPExpireAtWithArgs(ctx context.Context, key string, tm time.Time, expirationArgs redis.HExpireArgs, fields ...string) *redis.IntSliceCmd
	HPExpireTime(ctx context.Context, key string, fields ...string) *redis.IntSliceCmd
	HPExpireWithArgs(ctx context.Context, key string, expiration time.Duration, expirationArgs redis.HExpireArgs, fields ...string) *redis.IntSliceCmd
	HPTTL(ctx context.Context, key string, fields ...string) *redis.IntSliceCmd
	HPersist(ctx context.Context, key string, fields ...string) *redis.IntSliceCmd
	HRandField(ctx context.Context, key string, count int) *redis.StringSliceCmd
	HRandFieldWithValues(ctx context.Context, key string, count int) *redis.KeyValueSliceCmd
	HScan(ctx context.Context, key string, cursor uint64, match string, count int64) *redis.ScanCmd
	HScanNoValues(ctx context.Context, key string, cursor uint64, match string, count int64) *redis.ScanCmd
	HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	HSetNX(ctx context.Context, key, field string, value interface{}) *redis.BoolCmd
	HTTL(ctx context.Context, key string, fields ...string) *redis.IntSliceCmd
	HVals(ctx context.Context, key string) *redis.StringSliceCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
	IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd
	IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd
	Info(ctx context.Context, sections ...string) *redis.StringCmd
	InfoMap(ctx context.Context, sections ...string) *redis.InfoCmd
	JSONArrAppend(ctx context.Context, key, path string, values ...interface{}) *redis.IntSliceCmd
	JSONArrIndex(ctx context.Context, key, path string, value ...interface{}) *redis.IntSliceCmd
	JSONArrIndexWithArgs(ctx context.Context, key, path string, options *redis.JSONArrIndexArgs, value ...interface{}) *redis.IntSliceCmd
	JSONArrInsert(ctx context.Context, key, path string, index int64, values ...interface{}) *redis.IntSliceCmd
	JSONArrLen(ctx context.Context, key, path string) *redis.IntSliceCmd
	JSONArrPop(ctx context.Context, key, path string, index int) *redis.StringSliceCmd
	JSONArrTrim(ctx context.Context, key, path string) *redis.IntSliceCmd
	JSONArrTrimWithArgs(ctx context.Context, key, path string, options *redis.JSONArrTrimArgs) *redis.IntSliceCmd
	JSONClear(ctx context.Context, key, path string) *redis.IntCmd
	JSONDebugMemory(ctx context.Context, key, path string) *redis.IntCmd
	JSONDel(ctx context.Context, key, path string) *redis.IntCmd
	JSONForget(ctx context.Context, key, path string) *redis.IntCmd
	JSONGet(ctx context.Context, key string, paths ...string) *redis.JSONCmd
	JSONGetWithArgs(ctx context.Context, key string, options *redis.JSONGetArgs, paths ...string) *redis.JSONCmd
	JSONMGet(ctx context.Context, path string, keys ...string) *redis.JSONSliceCmd
	JSONMSet(ctx context.Context, params ...interface{}) *redis.StatusCmd
	JSONMSetArgs(ctx context.Context, docs []redis.JSONSetArgs) *redis.StatusCmd
	JSONMerge(ctx context.Context, key, path string, value string) *redis.StatusCmd
	JSONNumIncrBy(ctx context.Context, key, path string, value float64) *redis.JSONCmd
	JSONObjKeys(ctx context.Context, key, path string) *redis.SliceCmd
	JSONObjLen(ctx context.Context, key, path string) *redis.IntPointerSliceCmd
	JSONSet(ctx context.Context, key, path string, value interface{}) *redis.StatusCmd
	JSONSetMode(ctx context.Context, key, path string, value interface{}, mode string) *redis.StatusCmd
	JSONStrAppend(ctx context.Context, key, path, value string) *redis.IntPointerSliceCmd
	JSONStrLen(ctx context.Context, key, path string) *redis.IntPointerSliceCmd
	JSONToggle(ctx context.Context, key, path string) *redis.IntPointerSliceCmd
	JSONType(ctx context.Context, key, path string) *redis.JSONSliceCmd
	Keys(ctx context.Context, pattern string) *redis.StringSliceCmd
	LCS(ctx context.Context, q *redis.LCSQuery) *redis.LCSCmd
	LIndex(ctx context.Context, key string, index int64) *redis.StringCmd
	LInsert(ctx context.Context, key, op string, pivot, value interface{}) *redis.IntCmd
	LInsertAfter(ctx context.Context, key string, pivot, value interface{}) *redis.IntCmd
	LInsertBefore(ctx context.Context, key string, pivot, value interface{}) *redis.IntCmd
	LLen(ctx context.Context, key string) *redis.IntCmd
	LMPop(ctx context.Context, direction string, count int64, keys ...string) *redis.KeyValuesCmd
	LMove(ctx context.Context, source, destination, srcpos, destpos string) *redis.StringCmd
	LPop(ctx context.Context, key string) *redis.StringCmd
	LPopCount(ctx context.Context, key string, count int) *redis.StringSliceCmd
	LPos(ctx context.Context, key string, value string, a redis.LPosArgs) *redis.IntCmd
	LPosCount(ctx context.Context, key string, value string, count int64, a redis.LPosArgs) *redis.IntSliceCmd
	LPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	LPushX(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	LRem(ctx context.Context, key string, count int64, value interface{}) *redis.IntCmd
	LSet(ctx context.Context, key string, index int64, value interface{}) *redis.StatusCmd
	LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd
	LastSave(ctx context.Context) *redis.IntCmd
	MGet(ctx context.Context, keys ...string) *redis.SliceCmd
	MSet(ctx context.Context, values ...interface{}) *redis.StatusCmd
	MSetNX(ctx context.Context, values ...interface{}) *redis.BoolCmd
	MemoryUsage(ctx context.Context, key string, samples ...int) *redis.IntCmd
	Migrate(ctx context.Context, host, port, key string, db int, timeout time.Duration) *redis.StatusCmd
	ModuleLoadex(ctx context.Context, conf *redis.ModuleLoadexConfig) *redis.StringCmd
	Monitor(ctx context.Context, ch chan string) *redis.MonitorCmd
	Move(ctx context.Context, key string, db int) *redis.BoolCmd
	ObjectEncoding(ctx context.Context, key string) *redis.StringCmd
	ObjectFreq(ctx context.Context, key string) *redis.IntCmd
	ObjectIdleTime(ctx context.Context, key string) *redis.DurationCmd
	ObjectRefCount(ctx context.Context, key string) *redis.IntCmd
	Options() *redis.Options
	PExpire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	PExpireAt(ctx context.Context, key string, tm time.Time) *redis.BoolCmd
	PExpireTime(ctx context.Context, key string) *redis.DurationCmd
	PFAdd(ctx context.Context, key string, els ...interface{}) *redis.IntCmd
	PFCount(ctx context.Context, keys ...string) *redis.IntCmd
	PFMerge(ctx context.Context, dest string, keys ...string) *redis.StatusCmd
	PSubscribe(ctx context.Context, channels ...string) *redis.PubSub
	PTTL(ctx context.Context, key string) *redis.DurationCmd
	Persist(ctx context.Context, key string) *redis.BoolCmd
	Ping(ctx context.Context) *redis.StatusCmd
	Pipeline() redis.Pipeliner
	Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error)
	PoolStats() *redis.PoolStats
	Process(ctx context.Context, cmd redis.Cmder) error
	PubSubChannels(ctx context.Context, pattern string) *redis.StringSliceCmd
	PubSubNumPat(ctx context.Context) *redis.IntCmd
	PubSubNumSub(ctx context.Context, channels ...string) *redis.MapStringIntCmd
	PubSubShardChannels(ctx context.Context, pattern string) *redis.StringSliceCmd
	PubSubShardNumSub(ctx context.Context, channels ...string) *redis.MapStringIntCmd
	Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
	Quit(_ context.Context) *redis.StatusCmd
	RPop(ctx context.Context, key string) *redis.StringCmd
	RPopCount(ctx context.Context, key string, count int) *redis.StringSliceCmd
	RPopLPush(ctx context.Context, source, destination string) *redis.StringCmd
	RPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	RPushX(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	RandomKey(ctx context.Context) *redis.StringCmd
	ReadOnly(ctx context.Context) *redis.StatusCmd
	ReadWrite(ctx context.Context) *redis.StatusCmd
	Rename(ctx context.Context, key, newkey string) *redis.StatusCmd
	RenameNX(ctx context.Context, key, newkey string) *redis.BoolCmd
	Restore(ctx context.Context, key string, ttl time.Duration, value string) *redis.StatusCmd
	RestoreReplace(ctx context.Context, key string, ttl time.Duration, value string) *redis.StatusCmd
	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SCard(ctx context.Context, key string) *redis.IntCmd
	SDiff(ctx context.Context, keys ...string) *redis.StringSliceCmd
	SDiffStore(ctx context.Context, destination string, keys ...string) *redis.IntCmd
	SInter(ctx context.Context, keys ...string) *redis.StringSliceCmd
	SInterCard(ctx context.Context, limit int64, keys ...string) *redis.IntCmd
	SInterStore(ctx context.Context, destination string, keys ...string) *redis.IntCmd
	SIsMember(ctx context.Context, key string, member interface{}) *redis.BoolCmd
	SMIsMember(ctx context.Context, key string, members ...interface{}) *redis.BoolSliceCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	SMembersMap(ctx context.Context, key string) *redis.StringStructMapCmd
	SMove(ctx context.Context, source, destination string, member interface{}) *redis.BoolCmd
	SPop(ctx context.Context, key string) *redis.StringCmd
	SPopN(ctx context.Context, key string, count int64) *redis.StringSliceCmd
	SPublish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
	SRandMember(ctx context.Context, key string) *redis.StringCmd
	SRandMemberN(ctx context.Context, key string, count int64) *redis.StringSliceCmd
	SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SScan(ctx context.Context, key string, cursor uint64, match string, count int64) *redis.ScanCmd
	SSubscribe(ctx context.Context, channels ...string) *redis.PubSub
	SUnion(ctx context.Context, keys ...string) *redis.StringSliceCmd
	SUnionStore(ctx context.Context, destination string, keys ...string) *redis.IntCmd
	Save(ctx context.Context) *redis.StatusCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	ScanType(ctx context.Context, cursor uint64, match string, count int64, keyType string) *redis.ScanCmd
	ScriptExists(ctx context.Context, hashes ...string) *redis.BoolSliceCmd
	ScriptFlush(ctx context.Context) *redis.StatusCmd
	ScriptKill(ctx context.Context) *redis.StatusCmd
	ScriptLoad(ctx context.Context, script string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	SetArgs(ctx context.Context, key string, value interface{}, a redis.SetArgs) *redis.StatusCmd
	SetBit(ctx context.Context, key string, offset int64, value int) *redis.IntCmd
	SetEx(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	SetRange(ctx context.Context, key string, offset int64, value string) *redis.IntCmd
	SetXX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Shutdown(ctx context.Context) *redis.StatusCmd
	ShutdownNoSave(ctx context.Context) *redis.StatusCmd
	ShutdownSave(ctx context.Context) *redis.StatusCmd
	SlaveOf(ctx context.Context, host, port string) *redis.StatusCmd
	SlowLogGet(ctx context.Context, num int64) *redis.SlowLogCmd
	Sort(ctx context.Context, key string, sort *redis.Sort) *redis.StringSliceCmd
	SortInterfaces(ctx context.Context, key string, sort *redis.Sort) *redis.SliceCmd
	SortRO(ctx context.Context, key string, sort *redis.Sort) *redis.StringSliceCmd
	SortStore(ctx context.Context, key, store string, sort *redis.Sort) *redis.IntCmd
	StrLen(ctx context.Context, key string) *redis.IntCmd
	String() string
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
	Sync(_ context.Context)
	TDigestAdd(ctx context.Context, key string, elements ...float64) *redis.StatusCmd
	TDigestByRank(ctx context.Context, key string, rank ...uint64) *redis.FloatSliceCmd
	TDigestByRevRank(ctx context.Context, key string, rank ...uint64) *redis.FloatSliceCmd
	TDigestCDF(ctx context.Context, key string, elements ...float64) *redis.FloatSliceCmd
	TDigestCreate(ctx context.Context, key string) *redis.StatusCmd
	TDigestCreateWithCompression(ctx context.Context, key string, compression int64) *redis.StatusCmd
	TDigestInfo(ctx context.Context, key string) *redis.TDigestInfoCmd
	TDigestMax(ctx context.Context, key string) *redis.FloatCmd
	TDigestMerge(ctx context.Context, destKey string, options *redis.TDigestMergeOptions, sourceKeys ...string) *redis.StatusCmd
	TDigestMin(ctx context.Context, key string) *redis.FloatCmd
	TDigestQuantile(ctx context.Context, key string, elements ...float64) *redis.FloatSliceCmd
	TDigestRank(ctx context.Context, key string, values ...float64) *redis.IntSliceCmd
	TDigestReset(ctx context.Context, key string) *redis.StatusCmd
	TDigestRevRank(ctx context.Context, key string, values ...float64) *redis.IntSliceCmd
	TDigestTrimmedMean(ctx context.Context, key string, lowCutQuantile, highCutQuantile float64) *redis.FloatCmd
	TSAdd(ctx context.Context, key string, timestamp interface{}, value float64) *redis.IntCmd
	TSAddWithArgs(ctx context.Context, key string, timestamp interface{}, value float64, options *redis.TSOptions) *redis.IntCmd
	TSAlter(ctx context.Context, key string, options *redis.TSAlterOptions) *redis.StatusCmd
	TSCreate(ctx context.Context, key string) *redis.StatusCmd
	TSCreateRule(ctx context.Context, sourceKey string, destKey string, aggregator redis.Aggregator, bucketDuration int) *redis.StatusCmd
	TSCreateRuleWithArgs(ctx context.Context, sourceKey string, destKey string, aggregator redis.Aggregator, bucketDuration int, options *redis.TSCreateRuleOptions) *redis.StatusCmd
	TSCreateWithArgs(ctx context.Context, key string, options *redis.TSOptions) *redis.StatusCmd
	TSDecrBy(ctx context.Context, Key string, timestamp float64) *redis.IntCmd
	TSDecrByWithArgs(ctx context.Context, key string, timestamp float64, options *redis.TSIncrDecrOptions) *redis.IntCmd
	TSDel(ctx context.Context, Key string, fromTimestamp int, toTimestamp int) *redis.IntCmd
	TSDeleteRule(ctx context.Context, sourceKey string, destKey string) *redis.StatusCmd
	TSGet(ctx context.Context, key string) *redis.TSTimestampValueCmd
	TSGetWithArgs(ctx context.Context, key string, options *redis.TSGetOptions) *redis.TSTimestampValueCmd
	TSIncrBy(ctx context.Context, Key string, timestamp float64) *redis.IntCmd
	TSIncrByWithArgs(ctx context.Context, key string, timestamp float64, options *redis.TSIncrDecrOptions) *redis.IntCmd
	TSInfo(ctx context.Context, key string) *redis.MapStringInterfaceCmd
	TSInfoWithArgs(ctx context.Context, key string, options *redis.TSInfoOptions) *redis.MapStringInterfaceCmd
	TSMAdd(ctx context.Context, ktvSlices [][]interface{}) *redis.IntSliceCmd
	TSMGet(ctx context.Context, filters []string) *redis.MapStringSliceInterfaceCmd
	TSMGetWithArgs(ctx context.Context, filters []string, options *redis.TSMGetOptions) *redis.MapStringSliceInterfaceCmd
	TSMRange(ctx context.Context, fromTimestamp int, toTimestamp int, filterExpr []string) *redis.MapStringSliceInterfaceCmd
	TSMRangeWithArgs(ctx context.Context, fromTimestamp int, toTimestamp int, filterExpr []string, options *redis.TSMRangeOptions) *redis.MapStringSliceInterfaceCmd
	TSMRevRange(ctx context.Context, fromTimestamp int, toTimestamp int, filterExpr []string) *redis.MapStringSliceInterfaceCmd
	TSMRevRangeWithArgs(ctx context.Context, fromTimestamp int, toTimestamp int, filterExpr []string, options *redis.TSMRevRangeOptions) *redis.MapStringSliceInterfaceCmd
	TSQueryIndex(ctx context.Context, filterExpr []string) *redis.StringSliceCmd
	TSRange(ctx context.Context, key string, fromTimestamp int, toTimestamp int) *redis.TSTimestampValueSliceCmd
	TSRangeWithArgs(ctx context.Context, key string, fromTimestamp int, toTimestamp int, options *redis.TSRangeOptions) *redis.TSTimestampValueSliceCmd
	TSRevRange(ctx context.Context, key string, fromTimestamp int, toTimestamp int) *redis.TSTimestampValueSliceCmd
	TSRevRangeWithArgs(ctx context.Context, key string, fromTimestamp int, toTimestamp int, options *redis.TSRevRangeOptions) *redis.TSTimestampValueSliceCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
	Time(ctx context.Context) *redis.TimeCmd
	TopKAdd(ctx context.Context, key string, elements ...interface{}) *redis.StringSliceCmd
	TopKCount(ctx context.Context, key string, elements ...interface{}) *redis.IntSliceCmd
	TopKIncrBy(ctx context.Context, key string, elements ...interface{}) *redis.StringSliceCmd
	TopKInfo(ctx context.Context, key string) *redis.TopKInfoCmd
	TopKList(ctx context.Context, key string) *redis.StringSliceCmd
	TopKListWithCount(ctx context.Context, key string) *redis.MapStringIntCmd
	TopKQuery(ctx context.Context, key string, elements ...interface{}) *redis.BoolSliceCmd
	TopKReserve(ctx context.Context, key string, k int64) *redis.StatusCmd
	TopKReserveWithOptions(ctx context.Context, key string, k int64, width, depth int64, decay float64) *redis.StatusCmd
	Touch(ctx context.Context, keys ...string) *redis.IntCmd
	TxPipeline() redis.Pipeliner
	TxPipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error)
	Type(ctx context.Context, key string) *redis.StatusCmd
	Unlink(ctx context.Context, keys ...string) *redis.IntCmd
	Wait(ctx context.Context, numSlaves int, timeout time.Duration) *redis.IntCmd
	WaitAOF(ctx context.Context, numLocal, numSlaves int, timeout time.Duration) *redis.IntCmd
	Watch(ctx context.Context, fn func(*redis.Tx) error, keys ...string) error
	WithTimeout(timeout time.Duration) *redis.Client
	XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd
	XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd
	XAutoClaim(ctx context.Context, a *redis.XAutoClaimArgs) *redis.XAutoClaimCmd
	XAutoClaimJustID(ctx context.Context, a *redis.XAutoClaimArgs) *redis.XAutoClaimJustIDCmd
	XClaim(ctx context.Context, a *redis.XClaimArgs) *redis.XMessageSliceCmd
	XClaimJustID(ctx context.Context, a *redis.XClaimArgs) *redis.StringSliceCmd
	XDel(ctx context.Context, stream string, ids ...string) *redis.IntCmd
	XGroupCreate(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XGroupCreateConsumer(ctx context.Context, stream, group, consumer string) *redis.IntCmd
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XGroupDelConsumer(ctx context.Context, stream, group, consumer string) *redis.IntCmd
	XGroupDestroy(ctx context.Context, stream, group string) *redis.IntCmd
	XGroupSetID(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XInfoConsumers(ctx context.Context, key string, group string) *redis.XInfoConsumersCmd
	XInfoGroups(ctx context.Context, key string) *redis.XInfoGroupsCmd
	XInfoStream(ctx context.Context, key string) *redis.XInfoStreamCmd
	XInfoStreamFull(ctx context.Context, key string, count int) *redis.XInfoStreamFullCmd
	XLen(ctx context.Context, stream string) *redis.IntCmd
	XPending(ctx context.Context, stream, group string) *redis.XPendingCmd
	XPendingExt(ctx context.Context, a *redis.XPendingExtArgs) *redis.XPendingExtCmd
	XRange(ctx context.Context, stream, start, stop string) *redis.XMessageSliceCmd
	XRangeN(ctx context.Context, stream, start, stop string, count int64) *redis.XMessageSliceCmd
	XRead(ctx context.Context, a *redis.XReadArgs) *redis.XStreamSliceCmd
	XReadGroup(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd
	XReadStreams(ctx context.Context, streams ...string) *redis.XStreamSliceCmd
	XRevRange(ctx context.Context, stream, start, stop string) *redis.XMessageSliceCmd
	XRevRangeN(ctx context.Context, stream, start, stop string, count int64) *redis.XMessageSliceCmd
	XTrimMaxLen(ctx context.Context, key string, maxLen int64) *redis.IntCmd
	XTrimMaxLenApprox(ctx context.Context, key string, maxLen, limit int64) *redis.IntCmd
	XTrimMinID(ctx context.Context, key string, minID string) *redis.IntCmd
	XTrimMinIDApprox(ctx context.Context, key string, minID string, limit int64) *redis.IntCmd
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZAddArgs(ctx context.Context, key string, args redis.ZAddArgs) *redis.IntCmd
	ZAddArgsIncr(ctx context.Context, key string, args redis.ZAddArgs) *redis.FloatCmd
	ZAddGT(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZAddLT(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZAddNX(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZAddXX(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZCard(ctx context.Context, key string) *redis.IntCmd
	ZCount(ctx context.Context, key, min, max string) *redis.IntCmd
	ZDiff(ctx context.Context, keys ...string) *redis.StringSliceCmd
	ZDiffStore(ctx context.Context, destination string, keys ...string) *redis.IntCmd
	ZDiffWithScores(ctx context.Context, keys ...string) *redis.ZSliceCmd
	ZIncrBy(ctx context.Context, key string, increment float64, member string) *redis.FloatCmd
	ZInter(ctx context.Context, store *redis.ZStore) *redis.StringSliceCmd
	ZInterCard(ctx context.Context, limit int64, keys ...string) *redis.IntCmd
	ZInterStore(ctx context.Context, destination string, store *redis.ZStore) *redis.IntCmd
	ZInterWithScores(ctx context.Context, store *redis.ZStore) *redis.ZSliceCmd
	ZLexCount(ctx context.Context, key, min, max string) *redis.IntCmd
	ZMPop(ctx context.Context, order string, count int64, keys ...string) *redis.ZSliceWithKeyCmd
	ZMScore(ctx context.Context, key string, members ...string) *redis.FloatSliceCmd
	ZPopMax(ctx context.Context, key string, count ...int64) *redis.ZSliceCmd
	ZPopMin(ctx context.Context, key string, count ...int64) *redis.ZSliceCmd
	ZRandMember(ctx context.Context, key string, count int) *redis.StringSliceCmd
	ZRandMemberWithScores(ctx context.Context, key string, count int) *redis.ZSliceCmd
	ZRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	ZRangeArgs(ctx context.Context, z redis.ZRangeArgs) *redis.StringSliceCmd
	ZRangeArgsWithScores(ctx context.Context, z redis.ZRangeArgs) *redis.ZSliceCmd
	ZRangeByLex(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd
	ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd
	ZRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd
	ZRangeStore(ctx context.Context, dst string, z redis.ZRangeArgs) *redis.IntCmd
	ZRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd
	ZRank(ctx context.Context, key, member string) *redis.IntCmd
	ZRankWithScore(ctx context.Context, key, member string) *redis.RankWithScoreCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	ZRemRangeByLex(ctx context.Context, key, min, max string) *redis.IntCmd
	ZRemRangeByRank(ctx context.Context, key string, start, stop int64) *redis.IntCmd
	ZRemRangeByScore(ctx context.Context, key, min, max string) *redis.IntCmd
	ZRevRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	ZRevRangeByLex(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd
	ZRevRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd
	ZRevRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd
	ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd
	ZRevRank(ctx context.Context, key, member string) *redis.IntCmd
	ZRevRankWithScore(ctx context.Context, key, member string) *redis.RankWithScoreCmd
	ZScan(ctx context.Context, key string, cursor uint64, match string, count int64) *redis.ScanCmd
	ZScore(ctx context.Context, key, member string) *redis.FloatCmd
	ZUnion(ctx context.Context, store redis.ZStore) *redis.StringSliceCmd
	ZUnionStore(ctx context.Context, dest string, store *redis.ZStore) *redis.IntCmd
	ZUnionWithScores(ctx context.Context, store redis.ZStore) *redis.ZSliceCmd
}
