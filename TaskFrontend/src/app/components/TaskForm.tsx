import { useState } from "react";
import axios from "axios";

export default function TaskForm({ onTaskCreated }: { onTaskCreated: () => void }) {
  const [title, setTitle] = useState("");

  async function handleSubmit(e: any) {
    e.preventDefault();
    await axios.post(`${process.env.NEXT_PUBLIC_API_URL}/tasks`, { title, description: "" });
    setTitle("");
    onTaskCreated();
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-2">
      <input
        type="text"
        placeholder="Task Title"
        className="border p-2"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
      />
      <button type="submit" className="bg-green-500 text-white p-2">Add Task</button>
    </form>
  );
}
