const POINTS: [string, string][] = [
  ["encrypted here", "keys, hosts and files are encrypted in this browser before they're saved. the server never sees your password, questions, answers or keys."],
  ["password, then three questions", "opening the vault takes your password, then three questions you write, answered one at a time. each answer is part of the key."],
  ["no reset", "forget the password or an answer and the vault is gone for good. the export tab makes a decrypted copy for your own backup."],
  ["wrong answers add up", "every fifth wrong answer pauses unlocking for longer than the last, until it freezes and only whoever runs tuck can lift it."],
  ["it forgets quickly", "leaving or reloading the tab logs you out, and 2 quiet minutes lock the vault. the install tab turns it all into one script for ~/.ssh."],
];

export function Basics() {
  return (
    <ol className="basics">
      {POINTS.map(([title, body]) => (
        <li key={title}>
          <b>{title}:</b> {body}
        </li>
      ))}
    </ol>
  );
}
