// -*- mode: Bluespec; -*-
/**
 * Abstract Raft model for model-based testing against hashicorp/raft.
 *
 * This spec models a 3-node Raft cluster covering:
 *   1. Leader election safety (at most one leader per term)
 *   2. Log replication via a key-value FSM (set commands)
 *
 * The "grand daddy" Raft library — https://github.com/hashicorp/raft — is the
 * production implementation used in HashiCorp Consul, Nomad, and Vault.
 *
 * Actions
 * -------
 *  init           – all nodes start as followers in term 0; FSM is empty
 *  requestVote    – a follower starts an election by bumping its term
 *  grantVote      – a follower grants its vote to the candidate
 *  becomeLeader   – candidate with quorum becomes leader
 *  leaderCrash    – the current leader crashes (reverts to follower)
 *  applyCommand   – leader applies a set(key, value) command to the FSM
 *
 * Safety invariants
 * -----------------
 *  ElectionSafety: at most one leader per term
 *  AtMostOneLeader: at most one leader at any given moment
 */
module raft_hashicorp {
  type Role = Follower | Candidate | Leader

  /// The fixed set of nodes in the 3-node cluster.
  pure val NODES: Set[str] = Set("n1", "n2", "n3")
  pure val QUORUM: int = 2

  /// Key-value domain for the replicated FSM.
  pure val KEYS:   Set[str] = Set("k1", "k2", "k3")
  pure val VALUES: Set[str] = Set("v1", "v2", "v3")

  // --- Election state ---
  var roles:    str -> Role
  var terms:    str -> int       // currentTerm per node
  var votedFor: str -> str       // "" = not yet voted this term
  var votes:    str -> Set[str]  // votes received (Candidate only)

  // --- Replicated state machine ---
  var fsm: str -> str            // key-value store committed to the log

  action init = all {
    roles'    = NODES.mapBy(_ => Follower),
    terms'    = NODES.mapBy(_ => 0),
    votedFor' = NODES.mapBy(_ => ""),
    votes'    = NODES.mapBy(_ => Set()),
    fsm'      = Map(),
  }

  /// A follower starts an election: increments its term and votes for itself.
  action requestVote = {
    nondet node = NODES.oneOf()
    all {
      roles.get(node) == Follower,
      // Simplified: only one candidate at a time.
      NODES.forall(n => roles.get(n) != Candidate),
      roles'    = roles.set(node, Candidate),
      terms'    = terms.set(node, terms.get(node) + 1),
      votedFor' = votedFor.set(node, node),
      votes'    = votes.set(node, Set(node)),
      fsm'      = fsm,
    }
  }

  /// A follower grants its vote to the candidate for this term.
  action grantVote = {
    nondet voter     = NODES.oneOf()
    nondet candidate = NODES.oneOf()
    all {
      voter != candidate,
      roles.get(candidate) == Candidate,
      roles.get(voter) == Follower,
      votedFor.get(voter) == "",
      terms.get(candidate) >= terms.get(voter),
      roles'    = roles,
      terms'    = terms.set(voter, terms.get(candidate)),
      votedFor' = votedFor.set(voter, candidate),
      votes'    = votes.set(candidate, votes.get(candidate).union(Set(voter))),
      fsm'      = fsm,
    }
  }

  /// A candidate with a quorum of votes becomes leader.
  action becomeLeader = {
    nondet node = NODES.oneOf()
    all {
      roles.get(node) == Candidate,
      votes.get(node).size() >= QUORUM,
      roles'    = roles.set(node, Leader),
      terms'    = terms,
      votedFor' = votedFor,
      votes'    = votes,
      fsm'      = fsm,
    }
  }

  /// The current leader crashes and reverts to follower.
  action leaderCrash = {
    nondet node = NODES.oneOf()
    all {
      roles.get(node) == Leader,
      roles'    = roles.set(node, Follower),
      terms'    = terms,
      votedFor' = votedFor.set(node, ""),
      votes'    = votes.set(node, Set()),
      fsm'      = fsm,
    }
  }

  /// The leader applies a set(key, value) command; all nodes replicate it.
  action applyCommand = {
    nondet key   = KEYS.oneOf()
    nondet value = VALUES.oneOf()
    all {
      NODES.exists(n => roles.get(n) == Leader),
      roles'    = roles,
      terms'    = terms,
      votedFor' = votedFor,
      votes'    = votes,
      fsm'      = fsm.set(key, value),
    }
  }

  action step = any {
    requestVote,
    grantVote,
    becomeLeader,
    leaderCrash,
    applyCommand,
  }

  // -----------------------------------------------------------------------
  // Invariants
  // -----------------------------------------------------------------------

  /// At most one leader per term (Raft's core safety property).
  val ElectionSafety =
    NODES.forall(n1 =>
      NODES.forall(n2 =>
        (roles.get(n1) == Leader and roles.get(n2) == Leader and n1 != n2) implies
          terms.get(n1) != terms.get(n2)
      )
    )

  /// At most one leader in total at any moment.
  val AtMostOneLeader = NODES.filter(n => roles.get(n) == Leader).size() <= 1

  val inv = ElectionSafety and AtMostOneLeader
}
