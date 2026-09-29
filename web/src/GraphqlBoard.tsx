import { useMemo, useState } from 'react';
import { CombinedGraphQLErrors } from '@apollo/client';
import { ApolloProvider, useMutation, useQuery } from '@apollo/client/react';

import { STATUS_LABELS, type Task, type TaskStatus } from './api';
import { ADD_DEPENDENCY, PROGRAM_BOARD, SET_TASK_STATUS, createApolloClient } from './apollo';
import type { Board } from './board';
import { TaskBoard } from './TaskBoard';

// --- the shapes the graph returns -----------------------------------------

interface GraphUser {
  id: string;
  displayName: string;
}

interface GraphPredecessor {
  id: string;
  status: string;
}

interface GraphTask {
  id: string;
  title: string;
  status: string;
  estimateMinutes: number | null;
  dueOn: string | null;
  internalNote: string | null;
  assignee: GraphUser | null;
  predecessors: GraphPredecessor[];
}

interface ProgramBoardData {
  program: {
    id: string;
    name: string;
    tasks: GraphTask[];
  } | null;
}

/** toTask flattens a graph task into the same shape the REST board renders. */
function toTask(task: GraphTask, programId: string): Task {
  return {
    id: task.id,
    programId,
    orgId: '',
    title: task.title,
    status: task.status.toLowerCase() as TaskStatus,
    estimateMinutes: task.estimateMinutes,
    assigneeId: task.assignee?.id ?? null,
    dueOn: task.dueOn,
    internalNote: task.internalNote,
    createdAt: '',
    updatedAt: '',
  };
}

/**
 * isReady is phase 2's readiness rule, applied to what the cache already holds: a task is ready
 * when it is still todo and every task it waits on is done. Working it out here rather than
 * asking the server for a ready list is what keeps a downstream badge correct the moment a
 * mutation updates a predecessor's status, with no extra request.
 */
function isReady(task: GraphTask): boolean {
  return (
    task.status === 'TODO' && task.predecessors.every((predecessor) => predecessor.status === 'DONE')
  );
}

/** codeOf pulls the stable extensions.code out of whatever the mutation rejected with. */
function codeOf(error: unknown): string {
  if (error instanceof CombinedGraphQLErrors) {
    return String(error.errors[0]?.extensions?.['code'] ?? '');
  }
  return '';
}

export interface GraphqlBoardProps {
  programId: string;
}

/** GraphqlBoard owns the client, so every mount starts with an empty cache. */
export function GraphqlBoard({ programId }: GraphqlBoardProps) {
  const [client] = useState(createApolloClient);
  return (
    <ApolloProvider client={client}>
      <GraphqlBoardInner programId={programId} />
    </ApolloProvider>
  );
}

function GraphqlBoardInner({ programId }: GraphqlBoardProps) {
  const { data, loading, error } = useQuery<ProgramBoardData>(PROGRAM_BOARD, {
    variables: { id: programId },
  });
  const [setTaskStatus] = useMutation(SET_TASK_STATUS);
  const [addDependency] = useMutation(ADD_DEPENDENCY);

  const [announcement, setAnnouncement] = useState('');
  const [dependencyErrors, setDependencyErrors] = useState<Record<string, string>>({});

  const graphTasks = data?.program?.tasks ?? [];

  const board = useMemo<Board>(() => {
    const tasks = loading && data === undefined ? null : graphTasks.map((t) => toTask(t, programId));
    const readyIds = new Set(graphTasks.filter(isReady).map((t) => t.id));

    return {
      tasks,
      readyIds,
      loadError: error ? 'Could not load this program.' : null,
      announcement,
      dependencyErrors,
      changeStatus(task, status) {
        setAnnouncement('');
        void setTaskStatus({
          variables: { taskId: task.id, status: status.toUpperCase() },
          // Apollo shows this immediately and rolls it back itself if the server refuses.
          optimisticResponse: {
            setTaskStatus: { __typename: 'Task', id: task.id, status: status.toUpperCase() },
          },
        }).catch(() => {
          setAnnouncement(
            `Could not save "${task.title}" as ${STATUS_LABELS[status]}. It is back to ${STATUS_LABELS[task.status]}.`,
          );
        });
      },
      addTaskDependency(task, predecessorId) {
        setDependencyErrors((current) => ({ ...current, [task.id]: '' }));
        void addDependency({ variables: { taskId: task.id, predecessorId } }).catch(
          (failure: unknown) => {
            const predecessor = graphTasks.find((t) => t.id === predecessorId);
            const name = predecessor ? `"${predecessor.title}"` : 'that task';
            const message =
              codeOf(failure) === 'CYCLE'
                ? `${name} already has to wait for this one, so this would be a cycle. Nothing was changed.`
                : `Could not add ${name} as a dependency.`;
            setDependencyErrors((current) => ({ ...current, [task.id]: message }));
            setAnnouncement(message);
          },
        );
      },
    };
  }, [
    graphTasks,
    programId,
    loading,
    data,
    error,
    announcement,
    dependencyErrors,
    setTaskStatus,
    addDependency,
  ]);

  return <TaskBoard board={board} />;
}
