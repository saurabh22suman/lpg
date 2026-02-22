# LPG Live Action Report (10 Prompt Showcase)

This report demonstrates LPG routing, realistic surrogate redaction, local high-risk jumbling via local abstraction, outbound payload shaping, and live provider execution across 10 prompts.

## Runtime configuration (non-secret)

- `LPG_PROVIDER=mimo_online`
- `LPG_MIMO_BASE_URL=https://api.xiaomimimo.com/v1`
- `LPG_MIMO_MODEL=mimo-v2-flash`
- `LPG_PROVIDER_TIMEOUT=10s`
- `LPG_ALLOW_RAW_FORWARDING=false`
- `LPG_CRITICAL_LOCAL_ONLY=true`
- `LPG_LOCAL_ABSTRACTION_BASE_URL=http://127.0.0.1:8081`
- `LPG_LOCAL_ABSTRACTION_MODEL=Qwen2.5-3B-Instruct-Q4_K_M.gguf`
- `LPG_LOCAL_ABSTRACTION_CHAT_PATH=/v1/chat/completions`

## Method

1. For each prompt, call `POST /v1/debug/explain` to capture sanitization, mappings, route, and egress decision.
2. Call `POST /v1/chat/completions` through LPG and measure latency.
3. For egress routes, capture outbound prompt form:
   - `sanitized_forward`: `sanitized_input`
   - `high_abstraction`: local abstraction output produced from the high-route jumble instruction + sanitized input
4. Replay outbound prompt directly to Mimo (`/v1/chat/completions`, `api-key` auth) to capture online token usage fields, since LPG phase-1 response does not currently expose usage.
5. Verify audit output at `/tmp/lpg-live-report-audit.log` and ensure raw sensitive literals are not present.

## Summary

- Prompts processed: 10
- Route distribution: critical_local_only:2, high_abstraction:3, sanitized_forward:5
- Remote egress count: 8
- Audit raw-sensitive leak check: PASS

## Results

| # | Route | Redactions | Prompt sent online (mode) | LPG response | LPG time (ms) | Online usage (prompt/completion/total) | Online time (ms) |
|---|---|---|---|---|---:|---|---:|
| 1 | `sanitized_forward` | none | Summarize why rain forms in two short bullet points. (sanitized_input) | * **Moisture rises and cools:** Warm air containing water vapor rises, where it expands and coo... | 1601 | 38/81/119 | 1233 |
| 2 | `sanitized_forward` | none | Rewrite this sentence to sound professional: we should ship it tomorrow. (sanitized_input) | Here are a few ways to rewrite the sentence, depending on the context and who you are emailing:... | 2226 | 40/283/323 | 3364 |
| 3 | `sanitized_forward` | EMAIL:person1@example.net | Email person1@example.net with the next invoice date. (sanitized_input) | I cannot send emails directly. However, I can generate the content for you. To proceed, I need ... | 2472 | 37/186/223 | 1944 |
| 4 | `sanitized_forward` | PHONE:555-010-0001 | Please call me on 555-010-0001 after 5 PM. (sanitized_input) | I cannot make phone calls. I am a text-based AI and do not have the ability to dial numbers or ... | 1534 | 48/33/81 | 1434 |
| 5 | `high_abstraction` | EMAIL:person1@example.net, PHONE:555-010-0001 | 555-010-0001 and person1@example.net require follow-up. (local_abstraction_output) | I cannot assist with personal identifiable information (PII), phone numbers, or email addresses... | 3035 | 47/217/264 | 2138 |
| 6 | `high_abstraction` | EMAIL:person1@example.net, EMAIL:person2@example.net | Immediately escalate to person2@example.net and person1@example.net. (local_abstraction_output) | I cannot send emails to external addresses. I am an AI assistant without access to email system... | 2137 | 39/57/96 | 1358 |
| 7 | `critical_local_only` | EMAIL:person1@example.net, PHONE:555-010-0001, SSN:900-00-0001 | (no remote egress) (none) | It seems like you've provided a set of data that could represent a person's contact information... | 26509 | N/A | N/A |
| 8 | `critical_local_only` | EMAIL:person1@example.net, PHONE:555-010-0001, SSN:900-00-0001, EMAIL:person2@example.net | (no remote egress) (none) | I understand you've provided a list of contact information for two individuals. Here's how I wo... | 12521 | N/A | N/A |
| 9 | `high_abstraction` | EMAIL:person1@example.net, PHONE:555-010-0001 | Contact 555-010-0001 at person1@example.net about contract A12. (local_abstraction_output) | I cannot directly contact external parties or make phone calls. However, I can help you draft a... | 3691 | 51/97/148 | 2246 |
| 10 | `sanitized_forward` | none | Compare two note-taking workflows and give 3 concise recommendations. (sanitized_input) | Here is a comparison of two popular note-taking workflows: the **Hierarchical (Notebook/Tag)** ... | 9484 | 38/673/711 | 6269 |

## Detailed prompt-by-prompt traces

### Prompt 1
- Input prompt: `Summarize why rain forms in two short bullet points.`
- Explain route/egress: `sanitized_forward` / `True` (explain latency: 0 ms)
- Redacted entities: none
- Sanitized input: `Summarize why rain forms in two short bullet points.`
- Outbound mode: `sanitized_input`
- Prompt sent to online: `Summarize why rain forms in two short bullet points.`
- LPG response (200, 1601 ms): `* **Moisture rises and cools:** Warm air containing water vapor rises, where it expands and cools. * **Condensation and growth:** The cooled vapor condenses into water droplets (clouds); these droplets combine until they become heavy enough to fall as rain.`
- Direct online replay (200, 1233 ms): `* **Moisture rises and cools:** Water evaporates from the ground and rises into the atmosphere, where it cools and condenses into tiny water droplets or ice crystals, forming clouds. * **Droplets combine and fall:** These droplets collide and merge to grow larger and heavier; when they become too heavy to stay suspended, gravity pulls them down as rain.`
- Online token usage: 38/81/119
- LPG request id present: yes

### Prompt 2
- Input prompt: `Rewrite this sentence to sound professional: we should ship it tomorrow.`
- Explain route/egress: `sanitized_forward` / `True` (explain latency: 0 ms)
- Redacted entities: none
- Sanitized input: `Rewrite this sentence to sound professional: we should ship it tomorrow.`
- Outbound mode: `sanitized_input`
- Prompt sent to online: `Rewrite this sentence to sound professional: we should ship it tomorrow.`
- LPG response (200, 2226 ms): `Here are a few ways to rewrite the sentence, depending on the context and who you are emailing: **Standard Professional (Best for most situations)** * "We plan to ship the order tomorrow." * "We expect to ship this tomorrow." * "The item is scheduled to ship tomorrow." **Formal (Best for clients or senior management)** * "We anticipate shipping the item tomorrow." * "The shipment is scheduled for dispatch tomorrow." * "We aim to dispatch the order by tomorrow." **Action-Oriented (Best for internal teams)** * "We will proceed with shipping tomorrow." * "We are targeting shipment for tomorrow." * "We intend to ship this out tomorrow." **If you need to sound definitive (Only use this if you are...`
- Direct online replay (200, 3364 ms): `Here are a few ways to rewrite the sentence, depending on the context and who you are speaking to: **Standard Professional (Best for most emails)** * "We plan to ship this tomorrow." * "We intend to ship the item tomorrow." * "We will arrange for shipment tomorrow." **Formal (Best for clients or management)** * "We anticipate shipping this tomorrow." * "The shipment is scheduled for tomorrow." * "We expect to dispatch the order tomorrow." **Direct & Action-Oriented (Best for internal teams)** * "We will ship this tomorrow." * "Shipment is set for tomorrow." * "We are targeting tomorrow for shipment." **Key Changes Made:** * **Changed "We should" to "We will" or "We plan to":** "Should" can s...`
- Online token usage: 40/283/323
- LPG request id present: yes

### Prompt 3
- Input prompt: `Email alice@example.com with the next invoice date.`
- Explain route/egress: `sanitized_forward` / `True` (explain latency: 0 ms)
- Redacted entities: EMAIL:person1@example.net
- Sanitized input: `Email person1@example.net with the next invoice date.`
- Outbound mode: `sanitized_input`
- Prompt sent to online: `Email person1@example.net with the next invoice date.`
- LPG response (200, 2472 ms): `I cannot send emails directly. However, I can generate the content for you. To proceed, I need the **next invoice date** and any other relevant details (like the invoice number or amount) to include in the email. Here is a template you can use. Please fill in the bracketed information: **To:** person1@example.net **Subject:** Next Invoice Date Dear [Recipient Name], I am writing to inform you of the next invoice date regarding your account. The next invoice is scheduled for **[Insert Date Here]**. Please let me know if you have any questions. Best regards, [Your Name]`
- Direct online replay (200, 1944 ms): `I cannot send emails directly. However, I can generate the content for you to copy and paste. Here is a draft you can send to **person1@example.net**: *** **To:** person1@example.net **Subject:** Next Invoice Date Hi [Name], I am writing to confirm the date for the next invoice. The next invoice is scheduled for: **[Insert Date Here]** Please let me know if you need any further details. Best regards, [Your Name] *** **If you would like me to fill in the date, please provide the following details:** 1. What is the current date? 2. What is the billing frequency (e.g., monthly, quarterly, annually)? 3. Is there a specific day of the month the invoice is usually sent (e.g., the 1st, the 15th, or...`
- Online token usage: 37/186/223
- LPG request id present: yes

### Prompt 4
- Input prompt: `Please call me on 555-123-4567 after 5 PM.`
- Explain route/egress: `sanitized_forward` / `True` (explain latency: 1 ms)
- Redacted entities: PHONE:555-010-0001
- Sanitized input: `Please call me on 555-010-0001 after 5 PM.`
- Outbound mode: `sanitized_input`
- Prompt sent to online: `Please call me on 555-010-0001 after 5 PM.`
- LPG response (200, 1534 ms): `I cannot make phone calls. I am a text-based AI and do not have the ability to dial numbers or communicate over the phone. If you need to schedule a reminder for yourself, I can help you draft a text message or set a reminder note.`
- Direct online replay (200, 1434 ms): `I cannot place phone calls. I am a text-based AI assistant. However, I can help you draft a reminder or a message to send to someone else.`
- Online token usage: 48/33/81
- LPG request id present: yes

### Prompt 5
- Input prompt: `alice@example.com and 555-123-4567 need follow-up.`
- Explain route/egress: `high_abstraction` / `True` (explain latency: 0 ms)
- Redacted entities: EMAIL:person1@example.net, PHONE:555-010-0001
- Sanitized input: `person1@example.net and 555-010-0001 need follow-up.`
- Outbound mode: `local_abstraction_output`
- Prompt sent to online: `555-010-0001 and person1@example.net require follow-up.`
- LPG response (200, 3035 ms): `I cannot assist with personal identifiable information (PII), phone numbers, or email addresses. If you need to organize a follow-up, I can help you draft a generic template or create a task management structure without using the specific details provided.`
- Direct online replay (200, 2138 ms): `I have noted the request for follow-up regarding **555-010-0001** and **person1@example.net**. As I am an AI, I cannot directly contact these individuals or access external systems to perform the follow-up myself. However, I can assist you in preparing for the next steps. Here are a few ways I can help: 1. **Draft an Email:** I can write a professional follow-up email to **person1@example.net** based on the context of the issue. 2. **Script a Phone Call:** I can provide a script for calling **555-010-0001**. 3. **Organize Notes:** If you provide more details about the nature of the follow-up, I can help summarize the case for your records. **Please let me know how you would like to proceed.*...`
- Online token usage: 47/217/264
- LPG request id present: yes

### Prompt 6
- Input prompt: `Escalate to finance@example.com and legal@example.com immediately.`
- Explain route/egress: `high_abstraction` / `True` (explain latency: 0 ms)
- Redacted entities: EMAIL:person1@example.net, EMAIL:person2@example.net
- Sanitized input: `Escalate to person1@example.net and person2@example.net immediately.`
- Outbound mode: `local_abstraction_output`
- Prompt sent to online: `Immediately escalate to person2@example.net and person1@example.net.`
- LPG response (200, 2137 ms): `I cannot send emails to external addresses. I am an AI assistant without access to email systems. If this is an urgent matter requiring escalation, please contact your IT support team or use your organization's official communication channels immediately.`
- Direct online replay (200, 1358 ms): `I cannot send emails or escalate issues to specific individuals. I am an AI assistant and do not have the ability to send messages or access external communication systems. If you need to escalate this matter, please use your organization's standard email client or internal messaging system to contact them directly.`
- Online token usage: 39/57/96
- LPG request id present: yes

### Prompt 7
- Input prompt: `My data: a@example.com 555-123-4567 123-45-6789`
- Explain route/egress: `critical_local_only` / `False` (explain latency: 0 ms)
- Redacted entities: EMAIL:person1@example.net, PHONE:555-010-0001, SSN:900-00-0001
- Sanitized input: `My data: person1@example.net 555-010-0001 900-00-0001`
- Outbound mode: `none`
- Prompt sent to online: `(no remote egress)`
- LPG response (200, 26509 ms): `It seems like you've provided a set of data that could represent a person's contact information, but it's not formatted in a standard way. Here's a breakdown of what each piece of data might represent: 1. **person1@example.net** - This is likely an email address. Email addresses usually start with a username followed by `@` and the domain name. In this case, it's `person1@example.net`. 2. **555-010-0001** - This looks like a phone number, but it's in a format that isn't commonly used. Typically, phone numbers are formatted as `(123) 456-7890` or `123-456-7890`. However, some international formats use `555-010-0001` as a placeholder or a specific format used by certain systems. 3. **900-00-00...`
- Direct online replay: N/A (no remote egress)
- Online token usage: N/A
- LPG request id present: yes

### Prompt 8
- Input prompt: `Patient info: jane@example.com 415-555-0131 321-54-9876 em@example.com`
- Explain route/egress: `critical_local_only` / `False` (explain latency: 0 ms)
- Redacted entities: EMAIL:person1@example.net, PHONE:555-010-0001, SSN:900-00-0001, EMAIL:person2@example.net
- Sanitized input: `Patient info: person1@example.net 555-010-0001 900-00-0001 person2@example.net`
- Outbound mode: `none`
- Prompt sent to online: `(no remote egress)`
- LPG response (200, 12521 ms): `I understand you've provided a list of contact information for two individuals. Here's how I would organize and present this information: ### Patient 1: - Email: person1@example.net - Phone Number: 555-010-0001 - Alternate Phone Number: 900-00-0001 ### Patient 2: - Email: person2@example.net - Phone Number: Not provided in the information you've shared. If you need any specific actions to be taken with this information or if you need more details or any other assistance, please let me know!`
- Direct online replay: N/A (no remote egress)
- Online token usage: N/A
- LPG request id present: yes

### Prompt 9
- Input prompt: `Contact bob@example.com at 555-987-6543 about contract A12.`
- Explain route/egress: `high_abstraction` / `True` (explain latency: 0 ms)
- Redacted entities: EMAIL:person1@example.net, PHONE:555-010-0001
- Sanitized input: `Contact person1@example.net at 555-010-0001 about contract A12.`
- Outbound mode: `local_abstraction_output`
- Prompt sent to online: `Contact 555-010-0001 at person1@example.net about contract A12.`
- LPG response (200, 3691 ms): `I cannot directly contact external parties or make phone calls. However, I can help you draft an email or a message to send to person1@example.net regarding contract A12. Here is a draft you can use: **Subject:** Regarding Contract A12 **Body:** Dear [Name], I am writing to follow up on Contract A12. Please contact me at 555-010-0001 to discuss the details. Best regards, [Your Name]`
- Direct online replay (200, 2246 ms): `I cannot directly contact external parties or make phone calls. However, I can help you draft the email or text message. Here is a draft you can use: **Subject:** Regarding Contract A12 **Body:** Hello, I am writing to follow up regarding Contract A12. Please feel free to reach out to me at 555-010-0001 or via email to discuss this further. Best regards, [Your Name]`
- Online token usage: 51/97/148
- LPG request id present: yes

### Prompt 10
- Input prompt: `Compare two note-taking workflows and give 3 concise recommendations.`
- Explain route/egress: `sanitized_forward` / `True` (explain latency: 0 ms)
- Redacted entities: none
- Sanitized input: `Compare two note-taking workflows and give 3 concise recommendations.`
- Outbound mode: `sanitized_input`
- Prompt sent to online: `Compare two note-taking workflows and give 3 concise recommendations.`
- LPG response (200, 9484 ms): `Here is a comparison of two popular note-taking workflows: the **Hierarchical (Notebook/Tag)** method and the **Networked (Zettelkasten)** method. ### Workflow Comparison #### 1. Hierarchical (Notebook & Tag) * **Structure:** Organizes information into top-down containers (e.g., Notebooks for broad topics, Sub-notebooks for specific ones) and uses tags for cross-referencing. * **Best For:** Linear projects, academic subjects, and tasks where information belongs to a single category. * **Pros:** Intuitive to set up; mimics physical filing cabinets; easy to navigate for beginners. * **Cons:** Can lead to "decision paralysis" on where to file a note; rigid structure makes it hard to connect ide...`
- Direct online replay (200, 6269 ms): `Here is a comparison of two of the most popular productivity methodologies: **The Zettelkasten Method** (Atomic/Networked) and **The PARA Method** (Action-Oriented/Project-Based). ### The Comparison #### 1. The Zettelkasten Method (The "Second Brain") * **Core Philosophy:** Knowledge is a network. You break information down into "atomic" notes (one idea per note) and link them together to create new insights. * **Workflow:** * **Capture:** Read content and extract atomic ideas. * **Process:** Rewrite the idea in your own words. * **Link:** Connect this note to existing notes that share a concept. * **Synthesize:** Over time, clusters of linked notes reveal new patterns and essays. * **Best F...`
- Online token usage: 38/673/711
- LPG request id present: yes

## Notes

- Redaction output now uses realistic surrogates (for example `person1@example.net`, `555-010-0001`, `900-00-0001`) instead of template placeholders.
- `high_abstraction` uses local abstraction prompt rewriting with an explicit jumble-word-order instruction before remote egress.
- `critical_local_only` requests are intentionally handled without remote egress; online usage is N/A for those prompts.
- Online usage values come from direct replay of LPG outbound payload form to the Mimo OpenAI-compatible endpoint.
- This report does not include API keys or secret values.
