export interface Player {
  socketID: string;
  name: string;
  isAlive: boolean;
  role?: string;
}

export interface ChatMessage {
  lobbyId: string;
  senderId: string;
  senderName: string;
  content: string;
  timestamp: string;
}
