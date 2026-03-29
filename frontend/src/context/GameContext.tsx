import { createContext, useContext, useEffect, useState } from "react";
import { sendEvent, onEvent, offEvent, getSocketId } from '../socket';
import { Player } from "../models/player";
import { useNavigate } from "react-router-dom";

type RoleConfig = Record<string, number>;

interface GameContextValue {
    // State
    lobbyId: string | null;
    isHost: boolean;
    players: Player[];
    name: string;
    setName: (n: string) => void;
    role: string | null;
    roleConfig: RoleConfig;
    setRoleConfig: (cfg: RoleConfig) => void;
    // Actions
    createLobby: () => void;
    joinLobby: (code: string) => void;
    startGame: () => void;
}

const GameContext = createContext<GameContextValue | null>(null);

export function GameProvider({ children }: { children: React.ReactNode }) {
    const navigate = useNavigate();

    const [lobbyId, setLobbyId] = useState<string | null>(null);
    const [isHost, setIsHost] = useState<boolean>(false);
    const [players, setPlayers] = useState<Player[]>([]);
    const [name, setName] = useState("");
    const [role, setRole] = useState<string | null>(null);
    const [roleConfig, setRoleConfig] = useState<RoleConfig>({});

    
    useEffect(() => {
        const onConnected = (data: { socketId: string }) => console.log('socket:', data.socketId);
        const onLobbyCreated = (data: { lobbyId: string; players: Player[] }) => {
            setLobbyId(data.lobbyId); 
            setIsHost(true); 
            setPlayers(data.players);
            navigate('/lobby');
        };
        
        const onPlayersUpdated = (data: { players: Player[] }) => setPlayers(data.players);
        const onGameStarted = () => console.log('game started');

        const onUserDisconnected = (data: { socketId: string }) =>
            setPlayers(prev => prev.filter(p => p.socketID !== data.socketId));

        const onRolesAssigned = (data: { role: string }) => {
            setRole(data.role);
            navigate('/game', { state: { role: data.role } });
        };

        onEvent('connected', onConnected);
        onEvent('lobby-created', onLobbyCreated);
        onEvent('players-updated', onPlayersUpdated);
        onEvent('game-started', onGameStarted);
        onEvent('user-disconnected', onUserDisconnected);
        onEvent('roles-assigned', onRolesAssigned);

        return () => {
            offEvent('connected', onConnected);
            offEvent('lobby-created', onLobbyCreated);
            offEvent('players-updated', onPlayersUpdated);
            offEvent('game-started', onGameStarted);
            offEvent('user-disconnected', onUserDisconnected);
            offEvent('roles-assigned', onRolesAssigned);
        };
    }, [navigate]);

    const createLobby = () => sendEvent("create-lobby", { name });
    
    const joinLobby = (code: string) =>
        sendEvent("join-lobby", { lobbyId: code, name });

    const startGame = () =>
        sendEvent("start-game", { lobbyId, roles: roleConfig });

    return (
        <GameContext.Provider value={{ lobbyId, isHost, players, name, setName, role, roleConfig, setRoleConfig, createLobby, joinLobby, startGame }}>
            {children}
        </GameContext.Provider>
    );
}

export function useGame() {
    const ctx = useContext(GameContext);
    if (!ctx) throw new Error('useGame must be used inside GameProvider');
    return ctx;
}