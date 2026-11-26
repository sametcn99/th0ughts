// Public post pages load only the reader's thread components.
import React from "react";
import { createRoot } from "react-dom/client";
import { Thread } from "./comments.jsx";

const root = document.getElementById("comments");
if (root) createRoot(root).render(<Thread />);
