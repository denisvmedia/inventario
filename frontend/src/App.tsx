import React, { useState } from 'react';
import AreaList from './components/AreaList';
import LocationList from './components/LocationList';

function App() {
  const [shouldFetchLocations] = useState(false);

  return (
    <div className="App">
      <AreaList />
      <LocationList shouldFetchLocations={shouldFetchLocations} />
    </div>
  );
}

export default App;
