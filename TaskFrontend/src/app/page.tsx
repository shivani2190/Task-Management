"use client"
import TaskList from "../app/components/TaskList";
import TaskForm from "../app/components/TaskForm";
import { getTasks } from "../app/services/tasks";
import { useEffect, useState } from "react";

export default function Home() {
  const [tasks, setTasks] = useState([]);

  useEffect(() => {
    loadTasks();
  }, []);

  async function loadTasks() {
    const data = await getTasks();
    setTasks(data);
  }

  return (
    <div className="container mx-auto p-4">
      <h1 className="text-3xl font-bold">Task Management</h1>
      <TaskForm onTaskCreated={loadTasks} />
      <TaskList tasks={tasks} />
    </div>
  );
}