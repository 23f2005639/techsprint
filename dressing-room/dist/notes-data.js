window.PRESENTER_NOTES = [
  {
    "title": "Why the store needs this",
    "duration": 40,
    "cue": "Ask: “What happens when two people want the last hoodie?” Point to the photo and crisis figures.",
    "paragraphs": [
      "Good morning judges. Hum The Dressing Room ke liye drop reservation system present kar rahe hain. Ye physical streetwear store hai, jahan limited hoodies aur sneakers ke liye customers online interest dikhate hain aur store mein pickup karte hain.",
      "Original problem mein double booking aur forgotten holds the. Updated brief mein demand bhi gir rahi hai: monthly revenue one-to-two lakh se sixty-to-eighty thousand, aur footfall sixty-five percent down hai. Ye organiser ke scenario numbers hain.",
      "Hamari solution hai: pehle demand samjho, phir available item safely hold karo, aur customer ko store pickup tak guide karo."
    ],
    "research": "[A] Original Problem 21 and [B] Crisis Adaptation Brief. Photo: Mediamodifier / Unsplash, illustrative product."
  },
  {
    "title": "Find trends and create share badges",
    "duration": 65,
    "cue": "Click “Match the post to a product”. Change the badge name or choose “Picked up”, then show the PNG button.",
    "paragraphs": [
      "Sabse pehle original brief ke social features. Captions, comments aur hashtags se product ke keywords match karte hain. Yahan OversizedHoodie ko product code HD-001 se match kiya hai. Product code simply hamara internal item identifier hai.",
      "Match button click karte hain. Sample mentions usual two se ten ho gaye, isliye item Trending Drop dikh raha hai. Ye sample signal hai. Sirf hype dekh kar purchase assume nahi karenge. Agli slide ka interest form size aur quantity clearly capture karega.",
      "Right side personalised social badge hai. Customer reserve kare ya pickup kare, uske naam ka Story image banega. Naam change karke dekhiye. Download PNG se ten-eighty by nineteen-twenty image milti hai. Real product mein successful reservation ya collection ke baad ye generate hoga. Yahan sample preview hai.",
      "Isse original brief ka trend parser aur social proof generator dono cover hote hain."
    ],
    "research": "[3] TwitterMonitor supports comparing activity with a baseline. Our alias matcher is a simple demonstration, not a complete social platform integration."
  },
  {
    "title": "Ask what people want before the drop",
    "duration": 55,
    "cue": "Select an item, size and quantity. Add sample interest and point to the changed bar.",
    "paragraphs": [
      "Updated brief ke liye ye pre-drop waitlist add ki hai. Customer item, size aur quantity batata hai. Staff launch se pehle summary dekh sakta hai.",
      "Main size L hoodie ki one unit add karta hoon. Bar badh gaya, lekin stock same hai. Is stage par customer ne sirf interest dikhaya hai. Staff actual supply aur requested sizes dekh kar drop plan karega.",
      "Chart units count karta hai, aur staff conversion metric customers count karta hai. Dono ko mix nahi karenge. Hum popular size identify karte hain, instant replenishment promise nahi karte. Physical store ki limited-drop identity preserve hoti hai."
    ],
    "research": "[B] New requirements: item, size, quantity, and a demand summary before launch. All entries here are fictional demo data."
  },
  {
    "title": "Two customers request the last hoodie",
    "duration": 80,
    "cue": "Send both requests. After the packets arrive, click “Confirm one hold”. Point to the receipt and countdown. Swap order if time allows.",
    "paragraphs": [
      "Ab acceptance demo. Same size L ki sirf one hoodie bachi hai. Aarav aur Meera exactly same millisecond par reserve karte hain. Send both requests click karta hoon.",
      "Dono requests same stock record par aayi. Database stock changes ko ek order mein handle karta hai. Pehli successful request stock check, stock minus one aur hold save ek saath karti hai. Isko transaction kehte hain. Beech mein doosri request item book nahi kar sakti.",
      "Confirm click ke baad Aarav ko hold mila. Meera ke stock check ko zero mila, toh woh waitlist mein rehti hai. Try Meera first se result reverse kar sakte hain. Time stamp ya customer ka naam safety guarantee nahi karta. Database rule karta hai.",
      "Winner ko digital receipt aur strict twenty-four-hour countdown milta hai, jo original brief mein required tha. Duplicate click par same request ID use karenge, isliye second hold nahi banega. Page ka animation explanation hai; real concurrency backend tests se prove karni hogi."
    ],
    "research": "[1] Silo informs competing-update validation. [5] PostgreSQL conditional row updates wait and re-check their stock condition. The proposed system does not implement Silo."
  },
  {
    "title": "Remind, collect or release, then notify next",
    "duration": 110,
    "cue": "Run reminder job → show directions → Advance to expiry. Then Reset hold → Mark collected. Read the next-customer notice aloud.",
    "paragraphs": [
      "Staff dashboard same hold ko continue karta hai. Demo clock ab booking ke twenty-two hours baad hai, yani pickup ke liye two hours baaki hain.",
      "Top row mein waitlist customers who held ka percentage, abhi uncollected holds, aur most-requested hoodie size dikhte hain. Conversion ka matlab confirmed hold mila, purchase complete hua ye claim nahi hai. Agar hum new interest add karein, summary update hoti hai.",
      "Run reminder job original brief ka scheduled two-hour reminder dikhata hai. Proposed backend SMS ya Push bhejega, jisme store directions hongi. Updated brief ke liye staff ko editable ready-to-send draft bhi milta hai. Is demo mein actual message send nahi hota. Map link example search hai; production mein verified store location configure karenge.",
      "Ab expiry par jaate hain. Held one se zero, available zero se one. Receipt invalid ho jaati hai. Worker same job dobara run kare tab bhi stock ek baar hi return hota hai. Neeche Meera ke liye priority notice ready hai, kyunki woh next in line hai. Notice milna confirmed hold nahi hai; usko claim karna hoga.",
      "Reset karke Mark collected dikhata hoon. Ab held sold ho gaya, available zero hi hai. Collected item expiry se return nahi hoga.",
      "Server saved deadline check karega. Customer browser close ho, tab bhi worker chalega. Missed job ko periodic database check catch karega. Is tarah expiry, queue routing, reminder aur new staff dashboard sab cover hote hain."
    ],
    "research": "[2] Leases supports rights with a deadline. [6] Redis expiry events may be delayed or missed, so saved database deadlines and guarded transitions remain authoritative."
  },
  {
    "title": "System architecture",
    "duration": 50,
    "cue": "Follow React → Go API → PostgreSQL, then Redis and Worker. Point to the walk-in rule and observed race. Open the deep explanation link for questions.",
    "paragraphs": [
      "Judges, online aur walk-in customers same physical stock ke liye compete karte hain. React aur TypeScript customer, staff aur benchmark screens banate hain. Go net/http API requests validate karti hai aur duplicate request par saved result return karti hai.",
      "PostgreSQL official stock register hai. pgxpool connections manage karta hai. Ek transaction hold, scan units, request result aur outbox save karta hai. Five available units mein maximum three online hold hote hain, kyunki two walk-in ke liye protected hain.",
      "Redis Lua request rush count karta hai. Separate Go worker Asynq se reminders, expiry aur FIFO offers process karta hai. Har action latest stored state check karta hai. Fitting unit physical return ke bina release nahi hoti.",
      "Go generator real concurrent HTTP requests bhejta hai. SSE benchmark dashboard actual latency, outcomes aur inventory checks dikhata hai. Hundred requests ke short race mein one unique hold aur ninety-nine stock conflicts mile. Detailed explanation aur ninety-second script slide ke link aur PowerPoint Notes mein hain."
    ],
    "research": "Implementation: dressing-room-live/internal/app, internal/load and web/src. Observed local benchmark: verification/benchmark-summary.json. HTML slides remain illustrative; the separate local Go application provides live operations."
  },
  {
    "title": "References and close",
    "duration": 20,
    "cue": "Leave slide 7 visible. Use the Q&A handout if judges ask for detail.",
    "paragraphs": [
      "Yahan dono problem statements, teen research papers ke downloaded PDFs aur official technical references hain. Hamari acceptance demo mein last hoodie ke liye one active hold milta hai, aur expiry par stock wapas available hota hai.",
      "Is solution se staff demand samajh sakta hai aur limited inventory better handle kar sakta hai. Thank you. We are ready for your questions."
    ],
    "research": "Research supports design principles. Demo results are not production performance or business outcome measurements."
  }
];
