import { useState, useEffect, useRef } from "react";
import { TextInput, Button } from "@mantine/core";
import { sendEvent, onEvent, offEvent } from "../socket";
import { ChatMessage } from "../models/player";

interface LobbyChatProps {
  lobbyId: string;
}

export default function LobbyChat({ lobbyId }: LobbyChatProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onChatHistory = (data: { messages: ChatMessage[] }) => {
      setMessages(data.messages);
    };

    const onChatMessage = (data: ChatMessage) => {
      setMessages((prev) => [...prev, data]);
    };

    onEvent("chat-history", onChatHistory);
    onEvent("chat-message", onChatMessage);

    return () => {
      offEvent("chat-history", onChatHistory);
      offEvent("chat-message", onChatMessage);
    };
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSend = () => {
    if (!input.trim()) return;
    sendEvent("chat-message", { lobbyId, content: input.trim() });
    setInput("");
  };

  return (
    <div
      style={{
        border: "1px solid #ccc",
        borderRadius: "4px",
        height: "300px",
        display: "flex",
        flexDirection: "column",
        marginTop: "16px",
      }}
    >
      <div style={{ flex: 1, overflowY: "auto", padding: "8px" }}>
        {messages.map((msg, i) => (
          <div key={i} style={{ marginBottom: "4px" }}>
            <strong>{msg.senderName}:</strong> {msg.content}
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>
      <div style={{ display: "flex", padding: "8px", gap: "8px" }}>
        <TextInput
          placeholder="Type a message..."
          value={input}
          onChange={(e) => setInput(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") handleSend();
          }}
          style={{ flex: 1 }}
        />
        <Button onClick={handleSend}>Send</Button>
      </div>
    </div>
  );
}
