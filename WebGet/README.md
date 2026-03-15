Implement the tool get_webpage_html(url) as improvement over WebFetch that doesnt handle JS rendered page

That tool produce:

get_webpage_html(url)  
   │  
   ├─ HTTP fetch  
   │  
   ├─ detect empty / SPA  
   │  
   ├─ detect anti-bot  
   │  
   └─ browser render  
