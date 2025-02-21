import axios from "axios";

const API_URL = process.env.NEXT_PUBLIC_API_URL;

export async function getTasks() {
  try {
    const res = await axios.get(`${API_URL}/tasks`);
    return res.data;
  } catch (error) {
    console.error("Failed to fetch tasks");
    return [];
  }
}
