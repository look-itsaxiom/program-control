import { ApolloClient, ApolloLink, HttpLink, InMemoryCache, gql } from '@apollo/client';

import { currentUserId } from './api';

/**
 * createApolloClient builds a client for one mount of the app. The cache is normalized by
 * `__typename` and `id`, which is the whole point of using Apollo here: a mutation that returns a
 * task updates that task everywhere it appears, including inside another task's predecessor list,
 * with no second request to find out what changed.
 */
export function createApolloClient(): ApolloClient {
  const identity = new ApolloLink((operation, forward) => {
    operation.setContext({ headers: { 'X-User-Id': currentUserId() } });
    return forward(operation);
  });

  return new ApolloClient({
    link: ApolloLink.from([identity, new HttpLink({ uri: '/graphql' })]),
    cache: new InMemoryCache(),
  });
}

/**
 * PROGRAM_BOARD asks for each task's predecessors' status as well as its own, so readiness is
 * something the client can work out from the cache rather than a server-computed list that would
 * go stale the moment a status changed.
 */
export const PROGRAM_BOARD = gql`
  query ProgramBoard($id: ID!) {
    program(id: $id) {
      id
      name
      tasks {
        id
        title
        status
        estimateMinutes
        dueOn
        internalNote
        assignee {
          id
          displayName
        }
        predecessors {
          id
          status
        }
      }
    }
  }
`;

export const SET_TASK_STATUS = gql`
  mutation SetTaskStatus($taskId: ID!, $status: TaskStatus!) {
    setTaskStatus(taskId: $taskId, status: $status) {
      id
      status
    }
  }
`;

export const ADD_DEPENDENCY = gql`
  mutation AddDependency($taskId: ID!, $predecessorId: ID!) {
    addDependency(taskId: $taskId, predecessorId: $predecessorId) {
      id
      predecessors {
        id
        status
      }
    }
  }
`;
