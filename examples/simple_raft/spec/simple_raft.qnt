module simple_raft {
  type Role = Follower | Candidate | Leader

  pure val NODES: Set[str] = Set("node0", "node1", "node2")
  pure val QUORUM: int = 2

  var roles: str -> Role
  var terms: str -> int
  var votedFor: str -> str
  var alive: str -> bool
  var votes: str -> Set[str]

  action init = all {
    roles' = NODES.mapBy(_ => Follower),
    terms' = NODES.mapBy(_ => 0),
    votedFor' = NODES.mapBy(_ => ""),
    alive' = NODES.mapBy(_ => true),
    votes' = NODES.mapBy(_ => Set()),
  }

  action triggerElection = {
    nondet node = NODES.oneOf()
    all {
      alive.get(node),
      roles.get(node) != Leader,
      roles' = roles.set(node, Candidate),
      terms' = terms.set(node, terms.get(node) + 1),
      votedFor' = votedFor.set(node, node),
      alive' = alive,
      votes' = votes.set(node, Set(node)),
    }
  }

  action grantVote = {
    nondet voter = NODES.oneOf()
    nondet candidate = NODES.oneOf()
    all {
      voter != candidate,
      alive.get(voter),
      alive.get(candidate),
      roles.get(voter) == Follower,
      roles.get(candidate) == Candidate,
      votedFor.get(voter) == "",
      terms.get(candidate) >= terms.get(voter),
      roles' = roles,
      terms' = terms.set(voter, terms.get(candidate)),
      votedFor' = votedFor.set(voter, candidate),
      alive' = alive,
      votes' = votes.set(candidate, votes.get(candidate).union(Set(voter))),
    }
  }

  action becomeLeader = {
    nondet node = NODES.oneOf()
    all {
      alive.get(node),
      roles.get(node) == Candidate,
      votes.get(node).size() >= QUORUM,
      roles' = roles.set(node, Leader),
      terms' = terms,
      votedFor' = votedFor,
      alive' = alive,
      votes' = votes,
    }
  }

  action crashLeader = {
    nondet node = NODES.oneOf()
    all {
      alive.get(node),
      roles.get(node) == Leader,
      roles' = roles.set(node, Follower),
      terms' = terms,
      votedFor' = votedFor.set(node, ""),
      alive' = alive.set(node, false),
      votes' = votes.set(node, Set()),
    }
  }

  action restartNode = {
    nondet node = NODES.oneOf()
    all {
      not(alive.get(node)),
      roles' = roles.set(node, Follower),
      terms' = terms,
      votedFor' = votedFor.set(node, ""),
      alive' = alive.set(node, true),
      votes' = votes.set(node, Set()),
    }
  }

  action step = any {
    triggerElection,
    grantVote,
    becomeLeader,
    crashLeader,
    restartNode,
  }

  val AtMostOneLeader =
    NODES.filter(n => alive.get(n) and roles.get(n) == Leader).size() <= 1

  val ElectionSafety =
    NODES.forall(n1 =>
      NODES.forall(n2 =>
        (alive.get(n1) and alive.get(n2) and roles.get(n1) == Leader and roles.get(n2) == Leader and n1 != n2)
          implies terms.get(n1) != terms.get(n2)
      )
    )

  val inv = AtMostOneLeader and ElectionSafety
}
