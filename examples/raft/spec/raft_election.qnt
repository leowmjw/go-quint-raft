// -*- mode: Bluespec; -*-
/**
 * Abstract Raft leader-election model for model-based testing with quint-connector-go.
 *
 * This spec models a simplified, 3-node Raft cluster focusing on the
 * leader-election safety property: at most one leader may exist at any term.
 *
 * Actions
 * -------
 *  init           – all nodes start as followers in term 0
 *  requestVote    – a follower/candidate starts an election by bumping its term
 *  grantVote      – a follower grants its vote to the candidate for this term
 *  becomeLeader   – a candidate with a quorum of votes becomes the leader
 *  leaderCrash    – the current leader crashes (steps down to follower)
 *
 * Safety invariant
 * ----------------
 *  At most one leader per term (ElectionSafety).
 */
module raft_election {
  type Role = Follower | Candidate | Leader

  /// The fixed set of nodes in a 3-node cluster.
  pure val NODES: Set[str] = Set("n1", "n2", "n3")
  pure val QUORUM: int = 2    // majority of 3

  var roles:    str -> Role
  var terms:    str -> int    // currentTerm for each node
  var votedFor: str -> str    // "" means has not voted in current term
  var votes:    str -> Set[str] // votes received (only meaningful for Candidate)

  action init = all {
    roles'    = NODES.mapBy(_ => Follower),
    terms'    = NODES.mapBy(_ => 0),
    votedFor' = NODES.mapBy(_ => ""),
    votes'    = NODES.mapBy(_ => Set()),
  }

  /// A follower starts an election: it increments its term and votes for itself.
  action requestVote = {
    nondet node = NODES.oneOf()
    all {
      roles.get(node) == Follower,
      // Only one candidate at a time in this simplified model.
      NODES.forall(n => roles.get(n) != Candidate),
      roles'    = roles.set(node, Candidate),
      terms'    = terms.set(node, terms.get(node) + 1),
      votedFor' = votedFor.set(node, node),
      votes'    = votes.set(node, Set(node)),
    }
  }

  /// A follower grants its vote to the candidate (if it hasn't voted this term
  /// and the candidate's term is at least as large).
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
    }
  }

  action step = any {
    requestVote,
    grantVote,
    becomeLeader,
    leaderCrash,
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

  /// At most one leader in total at any time.
  val AtMostOneLeader = NODES.filter(n => roles.get(n) == Leader).size() <= 1

  val inv = ElectionSafety and AtMostOneLeader
}
