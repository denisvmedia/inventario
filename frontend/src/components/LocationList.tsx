import React, { useEffect, useState } from 'react';
import api from '../api/api';
import AddLocationForm from './AddLocationForm';

interface Location {
  id: string;
  name: string;
  address: string;
}

// interface LocationListProps {
//   shouldFetchLocations: boolean;
// }

function LocationList({ shouldFetchLocations }:any) {
  const [locations, setLocations] = useState<Location[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  const fetchLocations = async () => {
    try {
      const response = await api.get('/locations');
      setLocations(response.data.data);
      setIsLoading(false);
      // eslint-disable-next-line no-console
      console.log('here');
    } catch (error) {
      // eslint-disable-next-line no-console
      console.log('there');
      setError('Error fetching locations');
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchLocations();
  }, [shouldFetchLocations]); // Trigger the effect when shouldFetchLocations changes

  const handleLocationAdded = () => {
    // // Trigger re-fetching of locations
    // setLocations([]); // Clear the existing locations
    // setIsLoading(true); // Show loading state
    setError(''); // Clear any error messages
    fetchLocations(); // Fetch new locations
  };

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>{error}</div>;
  }

  return (
    <div>
      <h1>Location List</h1>
      <AddLocationForm onLocationAdded={handleLocationAdded} />
      {locations?.length > 0 ? (
        <ul>
          {locations.map((location) => (
            <li key={location.id}>
              <strong>Name:</strong>
              {' '}
              {location.name}
              ,
              {' '}
              <strong>Address:</strong>
              {' '}
              {location.address}
            </li>
          ))}
        </ul>
      ) : (
        <div>No locations found.</div>
      )}
    </div>
  );
}

export default LocationList;
