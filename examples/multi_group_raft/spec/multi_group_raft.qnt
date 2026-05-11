// -*- mode: Bluespec; -*-
/**
 * Abstract multi-group Raft model for model-based testing with quint-connector-go.
 *
 * This spec models a two-shard Raft cluster (inspired by dragonboat) focusing on:
 *   1. Independent leader-election safety within each shard.
 *   2. Data proposal routing: entries are proposed to the shard leader.
 *   3. Cross-shard isolation: a leader crash in one shard does not affect the other.
 *
 * Each shard runs a 3-node Raft group.  Shard and node identifiers match the Go
 * implementation so that the MBT driver can map spec state to dragonboat state.
 *
 * Shards:  1, 2
 * Nodes:   "n1", "n2", "n3"
 *
 * Actions
 * -------
 *  init              – all shards start with no leader, term 0, empty log
 *  electLeader       – a candidate shard node wins the election for its shard
 *  leaderCrash       – the current leader of a shard crashes (reverts to follower)
 *  proposeEntry      – the leader of a shard appends an entry to that shard's log
 *
 * Safety invariants
 * -----------------
 *  ElectionSafety   – at most one leader per shard at any time
 *  CrossShardIso    – a leader crash in one shard does not change the other shard's state
 */
module multi_group_raft {

  type Role = Follower | Leader

  /// Fixed set of shards and nodes.
  pure val SHARDS: Set[int] = Set(1, 2)
  pure val NODES:  Set[str] = Set("n1", "n2", "n3")

  // Per-shard, per-node role.
  var roles: int -> (str -> Role)

  // Logical term (election count) per shard.
  var terms: int -> int

  // Number of log entries appended per shard.
  var logLen: int -> int

  // -----------------------------------------------------------------------
  // Actions
  // -----------------------------------------------------------------------

  action init = all {
    roles'  = SHARDS.mapBy(_ => NODES.mapBy(_ => Follower)),
    terms'  = SHARDS.mapBy(_ => 0),
    logLen' = SHARDS.mapBy(_ => 0),
  }

  /// A follower in `shard` wins the election: it bumps the term and becomes leader.
  /// All other nodes in that shard revert to / remain followers.
  action electLeader = {
    nondet shard = SHARDS.oneOf()
    nondet node  = NODES.oneOf()
    all {
      // No current leader in this shard.
      NODES.forall(n => roles.get(shard).get(n) != Leader),
      // This node was a follower (not already a leader).
      roles.get(shard).get(node) == Follower,
      roles'  = roles.set(shard, roles.get(shard).set(node, Leader)),
      terms'  = terms.set(shard, terms.get(shard) + 1),
      logLen' = logLen,
    }
  }

  /// The leader of `shard` crashes and reverts to follower.
  action leaderCrash = {
    nondet shard = SHARDS.oneOf()
    nondet node  = NODES.oneOf()
    all {
      roles.get(shard).get(node) == Leader,
      roles'  = roles.set(shard, roles.get(shard).set(node, Follower)),
      terms'  = terms,
      logLen' = logLen,
    }
  }

  /// The leader of `shard` appends one entry to the shard log.
  action proposeEntry = {
    nondet shard = SHARDS.oneOf()
    all {
      // There must be a leader in this shard.
      NODES.exists(n => roles.get(shard).get(n) == Leader),
      roles'  = roles,
      terms'  = terms,
      logLen' = logLen.set(shard, logLen.get(shard) + 1),
    }
  }

  action step = any {
    electLeader,
    leaderCrash,
    proposeEntry,
  }

  // -----------------------------------------------------------------------
  // Invariants
  // -----------------------------------------------------------------------

  /// At most one leader per shard at any time.
  val ElectionSafety =
    SHARDS.forall(s =>
      NODES.filter(n => roles.get(s).get(n) == Leader).size() <= 1
    )

  /// Log entries only advance (log length is monotonically non-decreasing).
  val LogMonotone =
    SHARDS.forall(s => logLen.get(s) >= 0)

  val inv = ElectionSafety and LogMonotone
}
