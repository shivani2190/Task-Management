import { useEffect, useState } from "react";

export default function TaskList({ tasks }: { tasks: any[] }) {
  return (
    <ul>
      {tasks.map((task) => (
        <li key={task.id} className="p-2 border-b">
          <span>{task.title}</span> - <span>{task.status}</span>
        </li>
      ))}
    </ul>
  );
}
