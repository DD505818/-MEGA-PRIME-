import { useState } from 'react';
import { Panel } from './Panel';

export function LLMChatAssistant() {
  const [messages, setMessages] = useState([
    { role: 'assistant', text: 'Desk Copilot online. Ask for risk summaries or trade ideas.' },
  ]);
  const [input, setInput] = useState('');

  const send = () => {
    if (!input.trim()) return;
    setMessages((prev) => [
      ...prev,
      { role: 'user', text: input },
      { role: 'assistant', text: `Acknowledged: ${input}. Monitoring portfolio impact now.` },
    ]);
    setInput('');
  };

  return (
    <Panel title="LLM Chat Assistant" subtitle="Natural language ops">
      <div className="chat-box">
        {messages.map((msg, idx) => (
          <p key={idx} className={`msg ${msg.role}`}>{msg.text}</p>
        ))}
      </div>
      <div className="chat-input">
        <input value={input} onChange={(e) => setInput(e.target.value)} placeholder="Ask the assistant..." />
        <button onClick={send}>Send</button>
      </div>
    </Panel>
  );
}
