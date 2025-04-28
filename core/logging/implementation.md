# Simple Visitor and Country Counter Implementation

## Overview
This document outlines an optimized implementation for tracking total visitors and country visits using batch processing. The system will store this information in PocketBase collections with efficient batch updates.

## Database Collections

### visitors Collection
```json
{
  "ip": "string",
  "user_agent": "string",
  "country": "string",
  "country_code": "string",
  "timestamp": "date"
}
```

### visitor_stats Collection
```json
{
  "total_visitors": "number",
  "countries": {
    "US": 100,
    "GB": 50,
    "DE": 30
    // ... other countries
  },
  "last_updated": "date"
}
```

## Implementation Steps

1. **Database Setup**
   - Create visitors collection
   - Create visitor_stats collection with a single record for global stats
   - Add indexes on timestamp and country_code fields for efficient querying

2. **Middleware Implementation**
   - Create optimized visitor tracking middleware that:
     - Gets visitor IP and country information
     - Stores visitor data in a temporary buffer/cache
     - Implements batch processing:
       - Buffer visitor records for a short period (e.g., 5 minutes)
       - Perform bulk insert of visitor records
       - Update stats collection in a single operation
     - Use atomic operations for stats updates to prevent race conditions

3. **Batch Processing Strategy**
   - Implement a background worker that:
     - Processes buffered visitor records periodically
     - Aggregates country counts in memory
     - Performs bulk database operations
     - Updates stats collection with new totals
   - Use a sliding window approach for recent visitor data
   - Implement error handling and retry mechanisms for failed batch operations

4. **API Endpoints**

   #### GET /api/sys/stats
   - Returns:
     ```json
     {
       "total_visitors": 1000,
       "countries": {
         "US": 100,
         "GB": 50,
         "DE": 30
       },
       "last_updated": "2024-03-20T12:00:00Z"
     }
     ```

## Integration Points

1. **Index Route**
   - Add visitor tracking to the index route
   - Use non-blocking async operations for visitor recording

2. **Stats API**
   - Simple endpoint to get current stats
   - Include last update timestamp
   - Cache stats response for short periods (e.g., 1 minute)

## Performance Considerations

1. **Batch Size**
   - Configure optimal batch size based on system load
   - Consider memory constraints when buffering records
   - Implement backpressure handling for high traffic

2. **Update Frequency**
   - Balance between real-time accuracy and system load
   - Consider implementing different update frequencies for:
     - Recent visitors (more frequent updates)
     - Historical stats (less frequent updates)

3. **Error Handling**
   - Implement retry mechanisms for failed batch operations
   - Maintain audit logs for failed operations
   - Consider fallback strategies for high-load scenarios

## Next Steps

1. Create database collections with appropriate indexes
2. Implement visitor tracking middleware with batch processing
3. Set up background worker for batch operations
4. Add stats endpoint with caching
5. Update index route to show stats
6. Implement tests