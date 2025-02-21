import Link from "next/link";

export default function Navbar() {
  return (
    <nav className="bg-blue-600 text-white p-4">
      <div className="container mx-auto flex justify-between">
        <Link href="/" className="font-bold">Task Manager</Link>
        <div>
          <Link href="/login" className="mr-4">Login</Link>
          <Link href="/signup" className="mr-4">Signup</Link>
        </div>
      </div>
    </nav>
  );
}
